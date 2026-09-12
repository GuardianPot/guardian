package privileged

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	"github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
Converging one decoy container on its desired state.

The rules, and the direction each one fails in:

  - Stopping and removing never depend on the workload definition. An operator
    who uninstalls a definition and then asks for the decoy to go away must get
    that, so absent and stopped act on the workload id alone. Only running
    needs to know what to run.
  - A decoy does not start before the egress policy is in force on this boot.
    nftables state does not survive a reboot, and the component that starts
    decoys is the only place that ordering can be enforced. "Could not tell"
    refuses, the same way the P2-W1 conflict probe does.
  - A dead task under a desired running state is restarted on the next pass.
    That is the restart policy: the reconciler's cadence is the backoff, and
    the helper holds no crash-loop state of its own.
  - A spec that no longer matches the installed definition is replaced, not
    patched. The container records the digest of the spec it was built from,
    so an edited definition is noticed the next time anyone asks.
  - A start that fails is cleaned up in the same call: the task and container
    it left are removed, so the next pass begins from nothing rather than from
    half a decoy.

And the network (ADR 0019):

  - Every decoy has a holder, a Guardian container that owns the decoy's
    network namespace. The holder runs first, the host side is attached to it
    by pid, and the decoy joins its namespace.
  - The decoy's spec names the holder's pid, so a replaced holder changes the
    decoy's spec digest and the decoy is rebuilt into the new namespace. Holder
    and decoy restart together.
  - The pid a decoy joins is re-checked after the join and before the decoy's
    program runs. If the holder is not the same running task on both sides, the
    pid may belong to something else now, and the decoy is deleted unstarted.
  - Removal is the reverse of creation: decoy, host side, holder.
*/

const (
	// containerOperationTimeout bounds one ReconcileContainer call. It is long
	// because a first pass may have to fetch an image; every other privileged
	// operation stays at the helper's five seconds.
	containerOperationTimeout = 3 * time.Minute
	decoyStopGrace            = 5 * time.Second

	labelRole   = "guardian.role"
	roleHolder  = "network-holder"
	roleDecoy   = "decoy"
	reasonReady = "container-already-running"
)

var errHolderChanged = errors.New("holder-changed-before-start")

// decoyRuntimeAPI is the part of containerd the reconciler uses. It is an
// interface so the rules above can be tested without a runtime; the lab test
// exercises the real one.
type decoyRuntimeAPI interface {
	ensureImage(context.Context, Workload) (bool, *types.Descriptor, error)
	resolveImage(context.Context, *types.Descriptor) (resolvedImage, error)
	getContainer(context.Context, string) (*containersapi.Container, error)
	createContainer(context.Context, containerRecord) error
	deleteContainer(context.Context, *containersapi.Container) error
	getTask(context.Context, string) (*tasktypes.Process, error)
	startTask(context.Context, *containersapi.Container, func() error) error
	stopTask(context.Context, string, time.Duration) error
	deleteTask(context.Context, string) error
	Close() error
}

// decoyNetwork is the host side of ADR 0019: a veth into a running holder, a
// route to the decoy's /32, and forwarding on the zone interface. Both methods
// are idempotent and report whether they changed anything.
type decoyNetwork interface {
	attach(workload Workload, holderPID uint32) (bool, error)
	// detach works from the workload id alone, like stopping and removing.
	detach(workloadID string) (bool, error)
}

type containerRuntime struct {
	dial              func() (decoyRuntimeAPI, error)
	workloadDirectory string
	// egressReady reports nil only when the default-deny egress policy is
	// installed and matches the configured decoy ranges right now.
	egressReady func() error
	// allowsNetwork applies the root-controlled allowlist to the network a
	// definition names.
	allowsNetwork func(WorkloadNetwork) bool
	network       decoyNetwork
	holderBinary  string
	// verifyHolder refuses a holder binary anyone but root could have replaced.
	verifyHolder func(string) error
	// entrypoint lets the lab substitute a probe program for the image's own.
	// Production leaves it nil. Nothing reachable over the RPC can set it.
	entrypoint func(ImageEntrypoint) ImageEntrypoint
	// startFailed receives the runtime's own error when a start fails, so the
	// lab can show why. The RPC never carries that text; production leaves this
	// nil.
	startFailed func(error)
	stopGrace   time.Duration
}

func (r *containerRuntime) reconcile(ctx context.Context, operation ContainerOperation) (AdapterResult, error) {
	switch operation.DesiredState {
	case privilegedv1.ContainerState_CONTAINER_STATE_RUNNING:
		return r.run(ctx, operation.WorkloadID)
	case privilegedv1.ContainerState_CONTAINER_STATE_STOPPED:
		return r.stop(ctx, operation.WorkloadID)
	case privilegedv1.ContainerState_CONTAINER_STATE_ABSENT:
		return r.remove(ctx, operation.WorkloadID)
	default:
		return AdapterResult{}, violation(codes.InvalidArgument, "invalid-container-state")
	}
}

func (r *containerRuntime) run(ctx context.Context, workloadID string) (AdapterResult, error) {
	workload, err := LoadWorkload(r.workloadDirectory, workloadID)
	switch {
	case errors.Is(err, ErrWorkloadNotFound):
		return AdapterResult{}, violation(codes.FailedPrecondition, "workload-not-installed")
	case err != nil:
		return AdapterResult{}, violation(codes.FailedPrecondition, "workload-definition-invalid")
	}
	if r.allowsNetwork == nil || !r.allowsNetwork(workload.Network) {
		return AdapterResult{}, violation(codes.PermissionDenied, "workload-network-not-allowlisted")
	}
	if r.egressReady == nil || r.egressReady() != nil {
		return AdapterResult{}, violation(codes.FailedPrecondition, "egress-policy-not-applied")
	}
	if r.verifyHolder == nil || r.network == nil || r.verifyHolder(r.holderBinary) != nil {
		return AdapterResult{}, violation(codes.FailedPrecondition, "holder-binary-untrusted")
	}

	api, err := r.dial()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = api.Close() }()

	pulled, target, err := api.ensureImage(ctx, workload)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	image, err := api.resolveImage(ctx, target)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	entrypoint := image.entrypoint
	if r.entrypoint != nil {
		entrypoint = r.entrypoint(entrypoint)
	}

	holderPID, holderReplaced, err := r.ensureHolder(ctx, api, workload)
	if err != nil {
		return AdapterResult{}, err
	}
	attached, err := r.network.attach(workload, holderPID)
	if err != nil {
		return AdapterResult{}, networkError(err)
	}

	spec, err := BuildJoinedContainerSpec(workload, entrypoint, holderNamespacePath(holderPID))
	if errors.Is(err, ErrEntrypointInvalid) {
		return AdapterResult{}, violation(codes.FailedPrecondition, "image-entrypoint-invalid")
	}
	if err != nil {
		return AdapterResult{}, violation(codes.FailedPrecondition, "workload-definition-invalid")
	}
	specJSON, specDigest, err := encodeSpec(spec)
	if err != nil {
		return AdapterResult{}, err
	}

	reason := ""
	container, recreated, err := r.ensureContainer(ctx, api, containerRecord{
		ID: workloadID, Image: workload.ImageReference(), Parent: image.chainID, Spec: specJSON,
		Labels: map[string]string{
			labelWorkload: workloadID, labelSpecDigest: specDigest,
			labelImageDigest: workload.Image.Digest, labelRole: roleDecoy,
		},
	})
	if err != nil {
		return AdapterResult{}, err
	}
	if recreated {
		reason = "container-recreated"
	}
	if holderReplaced && recreated {
		reason = "holder-replaced"
	}

	process, err := api.getTask(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	// The join is re-checked against the holder the spec was built for.
	sameHolder := func() error {
		current, err := api.getTask(ctx, workload.HolderID())
		if err != nil || current == nil || current.GetStatus() != tasktypes.Status_RUNNING || current.GetPid() != holderPID {
			return errors.Join(err, errHolderChanged)
		}
		return nil
	}
	switch {
	case process != nil && process.GetStatus() == tasktypes.Status_RUNNING:
	case process == nil:
		if reason == "" {
			reason = "container-started"
		}
		if err := r.start(ctx, api, container, sameHolder, "container-start-failed"); err != nil {
			return AdapterResult{}, err
		}
	default:
		// Stopped, created but never started, paused, or unknown: under a
		// desired running state each of these is a decoy that is not doing its
		// job, and the fix for all of them is a fresh task.
		if err := api.stopTask(ctx, workloadID, r.grace()); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		if reason == "" {
			reason = "container-restarted"
		}
		if err := r.start(ctx, api, container, sameHolder, "container-start-failed"); err != nil {
			return AdapterResult{}, err
		}
	}

	switch {
	case reason != "":
		return applied(reason), nil
	case attached:
		return applied("network-reattached"), nil
	case pulled:
		return applied("image-pulled"), nil
	default:
		return unchanged(reasonReady), nil
	}
}

/*
ensureHolder converges the workload's holder and returns the pid of its running
task.

A holder that is not running is replaced together with everything that depends
on its namespace: the decoy's task is stopped and the host side detached before
a new holder starts, so a decoy is never left in a namespace nothing configures
and a veth is never left pointing at a namespace that is gone.
*/
func (r *containerRuntime) ensureHolder(ctx context.Context, api decoyRuntimeAPI, workload Workload) (uint32, bool, error) {
	spec, err := BuildHolderSpec(workload, r.holderBinary)
	if err != nil {
		return 0, false, violation(codes.FailedPrecondition, "workload-definition-invalid")
	}
	specJSON, specDigest, err := encodeSpec(spec)
	if err != nil {
		return 0, false, err
	}
	id := workload.HolderID()
	container, recreated, err := r.ensureContainer(ctx, api, containerRecord{
		ID: id, Spec: specJSON,
		Labels: map[string]string{labelWorkload: workload.WorkloadID, labelSpecDigest: specDigest, labelRole: roleHolder},
	})
	if err != nil {
		return 0, false, err
	}
	process, err := api.getTask(ctx, id)
	if err != nil {
		return 0, false, runtimeError(err)
	}
	if !recreated && process != nil && process.GetStatus() == tasktypes.Status_RUNNING && process.GetPid() > 1 {
		return process.GetPid(), false, nil
	}
	if err := api.stopTask(ctx, workload.WorkloadID, r.grace()); err != nil {
		return 0, false, runtimeError(err)
	}
	if _, err := r.network.detach(workload.WorkloadID); err != nil {
		return 0, false, networkError(err)
	}
	if process != nil {
		if err := api.stopTask(ctx, id, r.grace()); err != nil {
			return 0, false, runtimeError(err)
		}
	}
	if err := r.start(ctx, api, container, nil, "holder-start-failed"); err != nil {
		return 0, false, err
	}
	started, err := api.getTask(ctx, id)
	if err != nil {
		return 0, false, runtimeError(err)
	}
	if started == nil || started.GetStatus() != tasktypes.Status_RUNNING || started.GetPid() <= 1 {
		return 0, false, violation(codes.FailedPrecondition, "holder-start-failed")
	}
	// Replaced only if there was a holder task to replace. A holder started
	// after a stop, or for the first time, replaced nothing.
	return started.GetPid(), process != nil || recreated, nil
}

// ensureContainer returns the container for a record, creating it, or
// replacing one built from a different spec. A replaced container's task is
// stopped first.
func (r *containerRuntime) ensureContainer(ctx context.Context, api decoyRuntimeAPI, record containerRecord) (*containersapi.Container, bool, error) {
	container, err := api.getContainer(ctx, record.ID)
	if err != nil {
		return nil, false, runtimeError(err)
	}
	recreated := false
	if container != nil && container.GetLabels()[labelSpecDigest] != record.Labels[labelSpecDigest] {
		if err := api.stopTask(ctx, record.ID, r.grace()); err != nil {
			return nil, false, runtimeError(err)
		}
		if err := api.deleteContainer(ctx, container); err != nil {
			return nil, false, runtimeError(err)
		}
		container, recreated = nil, true
	}
	if container == nil {
		if err := api.createContainer(ctx, record); err != nil {
			return nil, false, runtimeError(err)
		}
		if container, err = api.getContainer(ctx, record.ID); err != nil || container == nil {
			return nil, false, runtimeError(errors.Join(err, errors.New("container-not-recorded")))
		}
	}
	return container, recreated, nil
}

func encodeSpec(spec ContainerSpec) ([]byte, string, error) {
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(specJSON)
	return specJSON, hex.EncodeToString(sum[:]), nil
}

// start starts a task, and on failure removes what the attempt left so the
// next pass is a clean one rather than a collision with half a decoy.
func (r *containerRuntime) start(ctx context.Context, api decoyRuntimeAPI, container *containersapi.Container, beforeStart func() error, reason string) error {
	err := api.startTask(ctx, container, beforeStart)
	if err == nil {
		return nil
	}
	if r.startFailed != nil {
		r.startFailed(err)
	}
	cleanup := context.WithoutCancel(ctx)
	_ = api.stopTask(cleanup, container.GetID(), r.grace())
	_ = api.deleteContainer(cleanup, container)
	if errors.Is(err, errHolderChanged) {
		return violation(codes.Aborted, "holder-changed-before-start")
	}
	return violation(codes.FailedPrecondition, reason)
}

// stop ends the decoy, then its host side, then its holder, and keeps both
// containers.
func (r *containerRuntime) stop(ctx context.Context, workloadID string) (AdapterResult, error) {
	api, err := r.dial()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = api.Close() }()
	changed, err := r.stopAll(ctx, api, workloadID)
	if err != nil {
		return AdapterResult{}, err
	}
	if changed {
		return applied("container-stopped"), nil
	}
	return unchanged("container-already-stopped"), nil
}

func (r *containerRuntime) stopAll(ctx context.Context, api decoyRuntimeAPI, workloadID string) (bool, error) {
	changed := false
	for _, id := range []string{workloadID, holderID(workloadID)} {
		process, err := api.getTask(ctx, id)
		if err != nil {
			return false, runtimeError(err)
		}
		if process != nil {
			if err := api.stopTask(ctx, id, r.grace()); err != nil {
				return false, runtimeError(err)
			}
			changed = changed || process.GetStatus() != tasktypes.Status_STOPPED
		}
		if id != workloadID || r.network == nil {
			continue
		}
		// The host side goes after the decoy and before the holder.
		detached, err := r.network.detach(workloadID)
		if err != nil {
			return false, networkError(err)
		}
		changed = changed || detached
	}
	return changed, nil
}

func (r *containerRuntime) remove(ctx context.Context, workloadID string) (AdapterResult, error) {
	api, err := r.dial()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = api.Close() }()
	changed, err := r.stopAll(ctx, api, workloadID)
	if err != nil {
		return AdapterResult{}, err
	}
	for _, id := range []string{workloadID, holderID(workloadID)} {
		container, err := api.getContainer(ctx, id)
		if err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		if container != nil {
			if err := api.deleteContainer(ctx, container); err != nil {
				return AdapterResult{}, runtimeError(err)
			}
			changed = true
		}
	}
	if changed {
		return applied("container-removed"), nil
	}
	return unchanged("container-already-absent"), nil
}

func (r *containerRuntime) grace() time.Duration {
	if r.stopGrace > 0 {
		return r.stopGrace
	}
	return decoyStopGrace
}

// runtimeError turns what containerd said into a closed reason. A runtime error
// message can name paths and digests that are not the caller's to see.
func runtimeError(err error) error {
	if _, ok := asViolation(err); ok {
		return err
	}
	switch {
	case errors.Is(err, errImageDigestMismatch):
		return violation(codes.FailedPrecondition, "image-digest-mismatch")
	case errors.Is(err, errImagePlatform):
		return violation(codes.FailedPrecondition, "image-platform-unavailable")
	case errors.Is(err, errImageMetadata):
		return violation(codes.FailedPrecondition, "image-metadata-invalid")
	}
	switch status.Code(err) {
	case codes.Unavailable:
		return violation(codes.Unavailable, "runtime-unreachable")
	case codes.DeadlineExceeded:
		return violation(codes.DeadlineExceeded, "runtime-timeout")
	}
	return err
}

// networkError keeps a typed refusal from the network side and closes
// everything else: a netlink errno says nothing an RPC caller needs.
func networkError(err error) error {
	if _, ok := asViolation(err); ok {
		return err
	}
	return violation(codes.Internal, "network-attach-failed")
}
