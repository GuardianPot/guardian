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
*/

const (
	// containerOperationTimeout bounds one ReconcileContainer call. It is long
	// because a first pass may have to fetch an image; every other privileged
	// operation stays at the helper's five seconds.
	containerOperationTimeout = 3 * time.Minute
	decoyStopGrace            = 5 * time.Second
)

// decoyRuntimeAPI is the part of containerd the reconciler uses. It is an
// interface so the rules above can be tested without a runtime; the lab test
// exercises the real one.
type decoyRuntimeAPI interface {
	ensureImage(context.Context, Workload) (bool, *types.Descriptor, error)
	resolveImage(context.Context, *types.Descriptor) (resolvedImage, error)
	getContainer(context.Context, string) (*containersapi.Container, error)
	createContainer(context.Context, Workload, []byte, string, string) error
	deleteContainer(context.Context, *containersapi.Container) error
	getTask(context.Context, string) (*tasktypes.Process, error)
	startTask(context.Context, *containersapi.Container) error
	stopTask(context.Context, string, time.Duration) error
	deleteTask(context.Context, string) error
	Close() error
}

type containerRuntime struct {
	dial              func() (decoyRuntimeAPI, error)
	workloadDirectory string
	// egressReady reports nil only when the default-deny egress policy is
	// installed and matches the configured decoy ranges right now.
	egressReady func() error
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
	if r.egressReady == nil || r.egressReady() != nil {
		return AdapterResult{}, violation(codes.FailedPrecondition, "egress-policy-not-applied")
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
	spec, err := BuildContainerSpec(workload, entrypoint)
	if errors.Is(err, ErrEntrypointInvalid) {
		return AdapterResult{}, violation(codes.FailedPrecondition, "image-entrypoint-invalid")
	}
	if err != nil {
		return AdapterResult{}, violation(codes.FailedPrecondition, "workload-definition-invalid")
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return AdapterResult{}, err
	}
	specSum := sha256.Sum256(specJSON)
	specDigest := hex.EncodeToString(specSum[:])

	reason := ""
	container, err := api.getContainer(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	if container != nil && container.GetLabels()[labelSpecDigest] != specDigest {
		if err := api.stopTask(ctx, workloadID, r.grace()); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		if err := api.deleteContainer(ctx, container); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		container, reason = nil, "container-recreated"
	}
	if container == nil {
		if err := api.createContainer(ctx, workload, specJSON, specDigest, image.chainID); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		if container, err = api.getContainer(ctx, workloadID); err != nil || container == nil {
			return AdapterResult{}, runtimeError(errors.Join(err, errors.New("container-not-recorded")))
		}
	}

	process, err := api.getTask(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	switch {
	case process != nil && process.GetStatus() == tasktypes.Status_RUNNING:
	case process == nil:
		if reason == "" {
			reason = "container-started"
		}
		if err := r.start(ctx, api, container); err != nil {
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
		if err := r.start(ctx, api, container); err != nil {
			return AdapterResult{}, err
		}
	}

	switch {
	case reason != "":
		return applied(reason), nil
	case pulled:
		return applied("image-pulled"), nil
	default:
		return unchanged("container-already-running"), nil
	}
}

// start starts a task, and on failure removes what the attempt left so the
// next pass is a clean one rather than a collision with half a decoy.
func (r *containerRuntime) start(ctx context.Context, api decoyRuntimeAPI, container *containersapi.Container) error {
	err := api.startTask(ctx, container)
	if err == nil {
		return nil
	}
	if r.startFailed != nil {
		r.startFailed(err)
	}
	cleanup := context.WithoutCancel(ctx)
	_ = api.stopTask(cleanup, container.GetID(), r.grace())
	_ = api.deleteContainer(cleanup, container)
	return violation(codes.FailedPrecondition, "container-start-failed")
}

func (r *containerRuntime) stop(ctx context.Context, workloadID string) (AdapterResult, error) {
	api, err := r.dial()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = api.Close() }()
	process, err := api.getTask(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	if process == nil {
		return unchanged("container-already-stopped"), nil
	}
	wasRunning := process.GetStatus() != tasktypes.Status_STOPPED
	if err := api.stopTask(ctx, workloadID, r.grace()); err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	if wasRunning {
		return applied("container-stopped"), nil
	}
	return unchanged("container-already-stopped"), nil
}

func (r *containerRuntime) remove(ctx context.Context, workloadID string) (AdapterResult, error) {
	api, err := r.dial()
	if err != nil {
		return AdapterResult{}, err
	}
	defer func() { _ = api.Close() }()
	changed := false
	process, err := api.getTask(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	if process != nil {
		if err := api.stopTask(ctx, workloadID, r.grace()); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		changed = true
	}
	container, err := api.getContainer(ctx, workloadID)
	if err != nil {
		return AdapterResult{}, runtimeError(err)
	}
	if container != nil {
		if err := api.deleteContainer(ctx, container); err != nil {
			return AdapterResult{}, runtimeError(err)
		}
		changed = true
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
