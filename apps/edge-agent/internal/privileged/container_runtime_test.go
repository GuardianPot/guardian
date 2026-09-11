package privileged

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	privilegedv1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/privileged/gen/guardian/privileged/v1"
	containersapi "github.com/containerd/containerd/api/services/containers/v1"
	"github.com/containerd/containerd/api/types"
	tasktypes "github.com/containerd/containerd/api/types/task"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeRuntime is containerd reduced to the state the reconciler reads: which
// image is present, which containers and tasks exist, and what each call did.
type fakeRuntime struct {
	imagePresent  bool
	digestWrong   bool
	startFails    bool
	containers    map[string]*containersapi.Container
	tasks         map[string]*tasktypes.Process
	pulls, starts int
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{
		containers: map[string]*containersapi.Container{},
		tasks:      map[string]*tasktypes.Process{},
	}
}

func (f *fakeRuntime) ensureImage(_ context.Context, workload Workload) (bool, *types.Descriptor, error) {
	if f.digestWrong {
		return false, nil, errImageDigestMismatch
	}
	pulled := !f.imagePresent
	if pulled {
		f.pulls++
		f.imagePresent = true
	}
	return pulled, &types.Descriptor{Digest: workload.Image.Digest, MediaType: mediaTypeOCIIndex}, nil
}

func (f *fakeRuntime) resolveImage(context.Context, *types.Descriptor) (resolvedImage, error) {
	return resolvedImage{entrypoint: cowrieEntrypoint, chainID: testDigest}, nil
}

func (f *fakeRuntime) getContainer(_ context.Context, id string) (*containersapi.Container, error) {
	return f.containers[id], nil
}

func (f *fakeRuntime) createContainer(_ context.Context, workload Workload, _ []byte, specDigest, _ string) error {
	f.containers[workload.WorkloadID] = &containersapi.Container{
		ID:          workload.WorkloadID,
		Labels:      map[string]string{labelSpecDigest: specDigest},
		SnapshotKey: rootfsSnapshotPrefix + workload.WorkloadID,
	}
	return nil
}

func (f *fakeRuntime) deleteContainer(_ context.Context, container *containersapi.Container) error {
	delete(f.containers, container.GetID())
	return nil
}

func (f *fakeRuntime) getTask(_ context.Context, id string) (*tasktypes.Process, error) {
	return f.tasks[id], nil
}

func (f *fakeRuntime) startTask(_ context.Context, container *containersapi.Container) error {
	f.starts++
	if f.startFails {
		// A failed start can leave a created task behind, which is exactly
		// what the reconciler must clean up.
		f.tasks[container.GetID()] = &tasktypes.Process{Status: tasktypes.Status_CREATED}
		return errors.New("runc: exec: /cowrie/bin/cowrie: no such file or directory")
	}
	f.tasks[container.GetID()] = &tasktypes.Process{Status: tasktypes.Status_RUNNING, Pid: uint32(1000 + f.starts)}
	return nil
}

func (f *fakeRuntime) stopTask(_ context.Context, id string, _ time.Duration) error {
	delete(f.tasks, id)
	return nil
}

func (f *fakeRuntime) deleteTask(_ context.Context, id string) error {
	delete(f.tasks, id)
	return nil
}

func (f *fakeRuntime) Close() error { return nil }

type runtimeFixture struct {
	fake    *fakeRuntime
	runtime *containerRuntime
	dials   int
	egress  error
}

func newRuntimeFixture(t *testing.T, installed bool) *runtimeFixture {
	t.Helper()
	directory := t.TempDir()
	if installed {
		path := filepath.Join(directory, testWorkloadID+".json")
		if err := os.WriteFile(path, []byte(workloadJSON(nil)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture := &runtimeFixture{fake: newFakeRuntime()}
	fixture.runtime = &containerRuntime{
		dial: func() (decoyRuntimeAPI, error) {
			fixture.dials++
			return fixture.fake, nil
		},
		workloadDirectory: directory,
		egressReady:       func() error { return fixture.egress },
	}
	return fixture
}

func (f *runtimeFixture) reconcile(t *testing.T, state privilegedv1.ContainerState) (AdapterResult, error) {
	t.Helper()
	return f.runtime.reconcile(context.Background(), ContainerOperation{WorkloadID: testWorkloadID, DesiredState: state})
}

const (
	running = privilegedv1.ContainerState_CONTAINER_STATE_RUNNING
	stopped = privilegedv1.ContainerState_CONTAINER_STATE_STOPPED
	absent  = privilegedv1.ContainerState_CONTAINER_STATE_ABSENT
)

func expectResult(t *testing.T, result AdapterResult, err error, outcome privilegedv1.OperationOutcome, reason string) {
	t.Helper()
	if err != nil {
		t.Fatalf("err = %v, want %s", err, reason)
	}
	if result.Outcome != outcome || result.ReasonCode != reason {
		t.Fatalf("result = %+v, want %v %s", result, outcome, reason)
	}
}

func expectRefusal(t *testing.T, err error, reason string) {
	t.Helper()
	refusal, ok := asViolation(err)
	if !ok || refusal.ReasonCode != reason {
		t.Fatalf("err = %v, want refusal %s", err, reason)
	}
}

/*
 * No decoy starts before the egress policy is in force.
 *
 * This is the P2-W2 ordering, enforced by the only component that starts
 * decoys. The refusal happens before the runtime is even contacted: a decoy that
 * is never created cannot reach anything.
 */
func TestADecoyDoesNotStartBeforeItsEgressPolicy(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.egress = errors.New("egress-policy-not-applied")
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "egress-policy-not-applied")
	if fixture.dials != 0 || fixture.fake.pulls != 0 {
		t.Fatal("the runtime was contacted before egress was confirmed")
	}
}

// With no way to tell whether egress is enforced, the answer is no.
func TestAMissingEgressCheckRefusesToo(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.runtime.egressReady = nil
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "egress-policy-not-applied")
}

func TestAFreshHostPullsCreatesAndStartsOnceThenConverges(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-started")
	if fixture.fake.pulls != 1 || fixture.fake.starts != 1 {
		t.Fatalf("pulls=%d starts=%d, want one of each", fixture.fake.pulls, fixture.fake.starts)
	}
	// A reconciler converges repeatedly. The second pass must be a no-op, or
	// every pass would restart the decoy an attacker is talking to.
	result, err = fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, "container-already-running")
	if fixture.fake.pulls != 1 || fixture.fake.starts != 1 {
		t.Fatal("a converged decoy was pulled or started again")
	}
}

// The restart policy: a decoy that died under a desired running state is
// restarted on the next pass.
func TestADeadDecoyIsRestarted(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	fixture.fake.tasks[testWorkloadID].Status = tasktypes.Status_STOPPED
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-restarted")
	if fixture.fake.tasks[testWorkloadID].GetStatus() != tasktypes.Status_RUNNING {
		t.Fatal("the dead decoy was not restarted")
	}
}

// An edited definition is noticed and the container rebuilt from it, rather
// than a decoy continuing to run under a spec nobody installed any more.
func TestAnEditedDefinitionReplacesTheContainer(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	fixture.fake.containers[testWorkloadID].Labels[labelSpecDigest] = "built-from-an-older-definition"
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-recreated")
	if fixture.fake.containers[testWorkloadID].GetLabels()[labelSpecDigest] == "built-from-an-older-definition" {
		t.Fatal("the stale container was kept")
	}
}

// A start that fails leaves nothing behind, so the next pass starts clean.
func TestAFailedStartLeavesNothingBehind(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.fake.startFails = true
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "container-start-failed")
	if len(fixture.fake.containers) != 0 || len(fixture.fake.tasks) != 0 {
		t.Fatalf("left behind containers=%d tasks=%d", len(fixture.fake.containers), len(fixture.fake.tasks))
	}
	if refusal, _ := asViolation(err); strings.Contains(refusal.Error(), "cowrie") {
		t.Fatal("the runtime's error text reached the caller")
	}
}

// An image that is not the digest the workload pinned is never run.
func TestADifferentImageIsNeverRun(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.fake.digestWrong = true
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "image-digest-mismatch")
	if len(fixture.fake.containers) != 0 || fixture.fake.starts != 0 {
		t.Fatal("a mismatched image was used")
	}
}

func TestRunningNeedsAnInstalledDefinition(t *testing.T) {
	fixture := newRuntimeFixture(t, false)
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "workload-not-installed")
	if fixture.dials != 0 {
		t.Fatal("the runtime was contacted for a workload that is not installed")
	}
}

/*
 * Stopping and removing never depend on the definition.
 *
 * An operator who uninstalls a definition and then asks for the decoy to go
 * away must get that. A removal that required the thing it was removing to be
 * describable would leave decoys running that nobody can stop.
 */
func TestStoppingAndRemovingDoNotNeedTheDefinition(t *testing.T) {
	fixture := newRuntimeFixture(t, false)
	fixture.fake.containers[testWorkloadID] = &containersapi.Container{ID: testWorkloadID}
	fixture.fake.tasks[testWorkloadID] = &tasktypes.Process{Status: tasktypes.Status_RUNNING}

	result, err := fixture.reconcile(t, stopped)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-stopped")
	if _, ok := fixture.fake.containers[testWorkloadID]; !ok {
		t.Fatal("stopping removed the container")
	}
	result, err = fixture.reconcile(t, stopped)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, "container-already-stopped")

	result, err = fixture.reconcile(t, absent)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-removed")
	if len(fixture.fake.containers) != 0 || len(fixture.fake.tasks) != 0 {
		t.Fatal("removal left something behind")
	}
	result, err = fixture.reconcile(t, absent)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, "container-already-absent")
}

// What containerd says is reduced to a closed reason. Its messages name
// sockets, paths, and digests that are not the caller's to see.
func TestRuntimeErrorsBecomeClosedReasons(t *testing.T) {
	for reason, err := range map[string]error{
		"runtime-unreachable":        status.Error(codes.Unavailable, "dial unix /run/containerd/containerd.sock: connect: no such file"),
		"runtime-timeout":            status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
		"image-digest-mismatch":      errImageDigestMismatch,
		"image-platform-unavailable": errImagePlatform,
		"image-metadata-invalid":     errImageMetadata,
	} {
		refusal, ok := asViolation(runtimeError(err))
		if !ok || refusal.ReasonCode != reason {
			t.Fatalf("%v became %v, want %s", err, runtimeError(err), reason)
		}
	}
}

func TestChainIDFollowsTheOCIRule(t *testing.T) {
	first := "sha256:" + strings.Repeat("a", 64)
	second := "sha256:" + strings.Repeat("b", 64)
	if got, err := chainIDOf([]string{first}); err != nil || got != first {
		t.Fatalf("one layer = %q %v, want the diff id itself", got, err)
	}
	// Computed independently: sha256 of "<first> <second>".
	const want = "sha256:ccd722928bd92476ba1745586fed6e45a102504185ad88cd89e01ff116fd146c"
	if got, err := chainIDOf([]string{first, second}); err != nil || got != want {
		t.Fatalf("two layers = %q %v, want %s", got, err, want)
	}
	if _, err := chainIDOf([]string{"not-a-digest"}); err == nil {
		t.Fatal("a malformed diff id was accepted")
	}
}
