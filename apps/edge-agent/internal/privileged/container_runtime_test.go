package privileged

import (
	"context"
	"encoding/json"
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
	specs         map[string][]byte
	tasks         map[string]*tasktypes.Process
	pulls, starts int
	// betweenCreateAndStart runs where a real runtime has created a task's
	// namespaces and not yet run its program.
	betweenCreateAndStart func()
	calls                 []string
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{
		containers: map[string]*containersapi.Container{},
		specs:      map[string][]byte{},
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

func (f *fakeRuntime) createContainer(_ context.Context, record containerRecord) error {
	labels := map[string]string{}
	for key, value := range record.Labels {
		labels[key] = value
	}
	f.containers[record.ID] = &containersapi.Container{
		ID:          record.ID,
		Labels:      labels,
		SnapshotKey: rootfsSnapshotPrefix + record.ID,
	}
	f.specs[record.ID] = record.Spec
	return nil
}

func (f *fakeRuntime) deleteContainer(_ context.Context, container *containersapi.Container) error {
	delete(f.containers, container.GetID())
	return nil
}

func (f *fakeRuntime) getTask(_ context.Context, id string) (*tasktypes.Process, error) {
	return f.tasks[id], nil
}

func (f *fakeRuntime) startTask(_ context.Context, container *containersapi.Container, beforeStart func() error) error {
	id := container.GetID()
	f.starts++
	if f.startFails && id == testWorkloadID {
		// A failed start can leave a created task behind, which is exactly
		// what the reconciler must clean up.
		f.tasks[id] = &tasktypes.Process{Status: tasktypes.Status_CREATED}
		return errors.New("runc: exec: /cowrie/bin/cowrie: no such file or directory")
	}
	if beforeStart != nil {
		if f.betweenCreateAndStart != nil {
			f.betweenCreateAndStart()
		}
		if err := beforeStart(); err != nil {
			return err
		}
	}
	f.calls = append(f.calls, "start "+id)
	f.tasks[id] = &tasktypes.Process{Status: tasktypes.Status_RUNNING, Pid: uint32(1000 + f.starts)}
	return nil
}

func (f *fakeRuntime) stopTask(_ context.Context, id string, _ time.Duration) error {
	if _, exists := f.tasks[id]; exists {
		f.calls = append(f.calls, "stop "+id)
	}
	delete(f.tasks, id)
	return nil
}

func (f *fakeRuntime) deleteTask(_ context.Context, id string) error {
	delete(f.tasks, id)
	return nil
}

func (f *fakeRuntime) Close() error { return nil }

// fakeNetwork is the host side reduced to which holder each workload is
// attached to.
type fakeNetwork struct {
	runtime  *fakeRuntime
	attached map[string]uint32
	err      error
}

func (n *fakeNetwork) attach(workload Workload, pid uint32) (bool, error) {
	if n.err != nil {
		return false, n.err
	}
	previous, ok := n.attached[workload.WorkloadID]
	n.attached[workload.WorkloadID] = pid
	if ok && previous == pid {
		return false, nil
	}
	n.runtime.calls = append(n.runtime.calls, "attach")
	return true, nil
}

func (n *fakeNetwork) detach(workloadID string) (bool, error) {
	_, ok := n.attached[workloadID]
	if ok {
		n.runtime.calls = append(n.runtime.calls, "detach")
	}
	delete(n.attached, workloadID)
	return ok, nil
}

type runtimeFixture struct {
	fake           *fakeRuntime
	network        *fakeNetwork
	runtime        *containerRuntime
	dials          int
	egress         error
	holderErr      error
	networkAllowed bool
}

var testHolderID = holderID(testWorkloadID)

func newRuntimeFixture(t *testing.T, installed bool) *runtimeFixture {
	t.Helper()
	directory := t.TempDir()
	if installed {
		path := filepath.Join(directory, testWorkloadID+".json")
		if err := os.WriteFile(path, []byte(workloadJSON(nil)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fake := newFakeRuntime()
	fixture := &runtimeFixture{
		fake:           fake,
		network:        &fakeNetwork{runtime: fake, attached: map[string]uint32{}},
		networkAllowed: true,
	}
	fixture.runtime = &containerRuntime{
		dial: func() (decoyRuntimeAPI, error) {
			fixture.dials++
			return fixture.fake, nil
		},
		workloadDirectory: directory,
		egressReady:       func() error { return fixture.egress },
		allowsNetwork:     func(WorkloadNetwork) bool { return fixture.networkAllowed },
		network:           fixture.network,
		holderBinary:      DefaultHolderBinary,
		verifyHolder:      func(string) error { return fixture.holderErr },
	}
	return fixture
}

func (f *runtimeFixture) reconcile(t *testing.T, state privilegedv1.ContainerState) (AdapterResult, error) {
	t.Helper()
	return f.runtime.reconcile(context.Background(), ContainerOperation{WorkloadID: testWorkloadID, DesiredState: state})
}

// joinedNamespace is the network namespace path the decoy's recorded spec joins.
func (f *runtimeFixture) joinedNamespace(t *testing.T) string {
	t.Helper()
	var spec ContainerSpec
	if err := json.Unmarshal(f.fake.specs[testWorkloadID], &spec); err != nil {
		t.Fatal(err)
	}
	path := ""
	for _, namespace := range spec.Linux.Namespaces {
		if namespace.Path == "" {
			continue
		}
		if namespace.Type != "network" || path != "" {
			t.Fatalf("the decoy joins %+v", spec.Linux.Namespaces)
		}
		path = namespace.Path
	}
	return path
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

// expectInOrder checks that the calls include these, in this order.
func expectInOrder(t *testing.T, calls []string, want ...string) {
	t.Helper()
	next := 0
	for _, call := range calls {
		if next < len(want) && call == want[next] {
			next++
		}
	}
	if next != len(want) {
		t.Fatalf("calls %v do not contain %v in order", calls, want)
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
	if fixture.dials != 0 || fixture.fake.pulls != 0 || len(fixture.network.attached) != 0 {
		t.Fatal("the runtime or the network was touched before egress was confirmed")
	}
}

// With no way to tell whether egress is enforced, the answer is no.
func TestAMissingEgressCheckRefusesToo(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.runtime.egressReady = nil
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "egress-policy-not-applied")
}

// A definition cannot attach a decoy to an interface or address the helper was
// not started with, and nothing is created when it tries.
func TestANetworkTheAllowlistDoesNotNameIsRefused(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.networkAllowed = false
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "workload-network-not-allowlisted")
	if fixture.dials != 0 {
		t.Fatal("the runtime was contacted for a network the allowlist does not name")
	}
}

// runc executes the holder binary with CAP_NET_ADMIN inside a decoy's namespace.
// One anyone but root could have replaced is never run.
func TestAnUntrustedHolderBinaryIsNeverRun(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.holderErr = ErrHolderBinaryUntrusted
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "holder-binary-untrusted")
	if fixture.dials != 0 {
		t.Fatal("the runtime was contacted with an untrusted holder binary")
	}
}

/*
 * ADR 0019, in order: the holder runs, the host side is attached to its pid,
 * and the decoy joins that pid's network namespace.
 */
func TestAFreshHostStartsTheHolderAttachesItAndJoinsTheDecoyToIt(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-started")
	if fixture.fake.pulls != 1 || fixture.fake.starts != 2 {
		t.Fatalf("pulls=%d starts=%d, want one pull and two starts", fixture.fake.pulls, fixture.fake.starts)
	}
	expectInOrder(t, fixture.fake.calls, "start "+testHolderID, "attach", "start "+testWorkloadID)
	holder := fixture.fake.tasks[testHolderID]
	if fixture.network.attached[testWorkloadID] != holder.GetPid() {
		t.Fatalf("attached to %d, holder is %d", fixture.network.attached[testWorkloadID], holder.GetPid())
	}
	if got, want := fixture.joinedNamespace(t), holderNamespacePath(holder.GetPid()); got != want {
		t.Fatalf("the decoy joins %q, want %q", got, want)
	}
	if fixture.fake.containers[testHolderID].GetLabels()[labelRole] != roleHolder {
		t.Fatal("the holder is not labelled as one")
	}

	// A reconciler converges repeatedly. The second pass must be a no-op, or
	// every pass would restart the decoy an attacker is talking to.
	result, err = fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, "container-already-running")
	if fixture.fake.pulls != 1 || fixture.fake.starts != 2 {
		t.Fatal("a converged decoy was pulled or started again")
	}
}

// The restart policy: a decoy that died under a desired running state is
// restarted on the next pass, in the same holder.
func TestADeadDecoyIsRestarted(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	holderPID := fixture.fake.tasks[testHolderID].GetPid()
	fixture.fake.tasks[testWorkloadID].Status = tasktypes.Status_STOPPED
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-restarted")
	if fixture.fake.tasks[testWorkloadID].GetStatus() != tasktypes.Status_RUNNING {
		t.Fatal("the dead decoy was not restarted")
	}
	if fixture.fake.tasks[testHolderID].GetPid() != holderPID {
		t.Fatal("a decoy crash replaced its holder")
	}
}

/*
 * Holder and decoy restart together.
 *
 * A dead holder's namespace survives while the decoy is still in it, with
 * nothing left to configure it. The decoy is stopped, the host side detached,
 * and both come back with the decoy in the new holder's namespace.
 */
func TestAReplacedHolderTakesItsDecoyWithIt(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	fixture.fake.tasks[testHolderID].Status = tasktypes.Status_STOPPED
	fixture.fake.calls = nil
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "holder-replaced")
	expectInOrder(t, fixture.fake.calls,
		"stop "+testWorkloadID, "detach", "stop "+testHolderID,
		"start "+testHolderID, "attach", "start "+testWorkloadID)
	holder := fixture.fake.tasks[testHolderID]
	if got := fixture.joinedNamespace(t); got != holderNamespacePath(holder.GetPid()) {
		t.Fatalf("the decoy joins %q, not the new holder %d", got, holder.GetPid())
	}
	if fixture.network.attached[testWorkloadID] != holder.GetPid() {
		t.Fatal("the host side still points at the old holder")
	}
}

// A holder started after a stop replaced nothing. The decoy is rebuilt, because
// its spec names the new holder's pid, and says so without claiming a holder
// died.
func TestStartingAfterAStopIsNotAHolderReplacement(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.reconcile(t, stopped); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-recreated")
	if got := fixture.joinedNamespace(t); got != holderNamespacePath(fixture.fake.tasks[testHolderID].GetPid()) {
		t.Fatalf("the decoy joins %q, not the running holder", got)
	}
}

/*
 * The pid a decoy joins is re-checked before its program runs.
 *
 * If the holder is not the same running task after the join, the pid may name
 * another process by now, and a decoy in that process's namespace could be on
 * the host's network. The decoy is deleted unstarted.
 */
func TestADecoyIsNeverStartedInANamespaceItsHolderNoLongerOwns(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.fake.betweenCreateAndStart = func() {
		fixture.fake.tasks[testHolderID] = &tasktypes.Process{Status: tasktypes.Status_RUNNING, Pid: 4242}
	}
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "holder-changed-before-start")
	if _, exists := fixture.fake.tasks[testWorkloadID]; exists {
		t.Fatal("the decoy's task survived a changed holder")
	}
	if _, exists := fixture.fake.containers[testWorkloadID]; exists {
		t.Fatal("the decoy's container survived a changed holder")
	}
	for _, call := range fixture.fake.calls {
		if call == "start "+testWorkloadID {
			t.Fatal("the decoy's program ran")
		}
	}
}

// The host side reattached behind a running decoy is a change worth reporting,
// and not a reason to restart anything.
func TestAMissingHostSideIsReattachedWithoutARestart(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	if _, err := fixture.reconcile(t, running); err != nil {
		t.Fatal(err)
	}
	delete(fixture.network.attached, testWorkloadID)
	result, err := fixture.reconcile(t, running)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "network-reattached")
	if fixture.fake.starts != 2 {
		t.Fatal("reattaching the host side restarted a task")
	}
}

// A failed attach starts nothing, and its refusal reaches the caller intact.
func TestAFailedAttachStartsNoDecoy(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.network.err = violation(codes.FailedPrecondition, "interface-not-found")
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "interface-not-found")
	if _, exists := fixture.fake.tasks[testWorkloadID]; exists {
		t.Fatal("a decoy started without its network")
	}
	fixture.network.err = errors.New("netlink: operation not permitted on gdnac1e1463")
	_, err = fixture.reconcile(t, running)
	expectRefusal(t, err, "network-attach-failed")
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

// A start that fails leaves no decoy behind, so the next pass starts clean.
func TestAFailedStartLeavesNothingBehind(t *testing.T) {
	fixture := newRuntimeFixture(t, true)
	fixture.fake.startFails = true
	_, err := fixture.reconcile(t, running)
	expectRefusal(t, err, "container-start-failed")
	if _, exists := fixture.fake.containers[testWorkloadID]; exists {
		t.Fatal("the failed start left a container")
	}
	if _, exists := fixture.fake.tasks[testWorkloadID]; exists {
		t.Fatal("the failed start left a task")
	}
	if refusal, _ := asViolation(err); strings.Contains(refusal.Error(), "cowrie") {
		t.Fatal("the runtime's error text reached the caller")
	}
}

// An image that is not the digest the workload pinned is never run, and no
// holder is started for it.
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
 * Stopping and removing never depend on the definition, and undo creation in
 * reverse: decoy, host side, holder.
 *
 * An operator who uninstalls a definition and then asks for the decoy to go
 * away must get that. A removal that required the thing it was removing to be
 * describable would leave decoys running that nobody can stop.
 */
func TestStoppingAndRemovingDoNotNeedTheDefinition(t *testing.T) {
	fixture := newRuntimeFixture(t, false)
	for _, id := range []string{testWorkloadID, testHolderID} {
		fixture.fake.containers[id] = &containersapi.Container{ID: id}
		fixture.fake.tasks[id] = &tasktypes.Process{Status: tasktypes.Status_RUNNING}
	}
	fixture.network.attached[testWorkloadID] = 1001

	result, err := fixture.reconcile(t, stopped)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-stopped")
	expectInOrder(t, fixture.fake.calls, "stop "+testWorkloadID, "detach", "stop "+testHolderID)
	if len(fixture.fake.containers) != 2 {
		t.Fatal("stopping removed a container")
	}
	result, err = fixture.reconcile(t, stopped)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_UNCHANGED, "container-already-stopped")

	result, err = fixture.reconcile(t, absent)
	expectResult(t, result, err, privilegedv1.OperationOutcome_OPERATION_OUTCOME_APPLIED, "container-removed")
	if len(fixture.fake.containers) != 0 || len(fixture.fake.tasks) != 0 || len(fixture.network.attached) != 0 {
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
