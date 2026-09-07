package reconciliation

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

const (
	edgeDeviceID       = "0198f7c4-7b30-7f11-8a44-111111111111"
	edgeEnvironmentID  = "0198f7c4-7b30-7f11-8a44-222222222222"
	edgeZoneID         = "0198f7c4-7b30-7f11-8a44-333333333333"
	edgeDecoyID        = "0198f7c4-7b30-7f11-8a44-aaaaaaaaaaaa"
	messageOne         = "0198f7c4-7b30-7f11-8a44-444444444444"
	messageThree       = "0198f7c4-7b30-7f11-8a44-555555555555"
	messageConflict    = "0198f7c4-7b30-7f11-8a44-666666666666"
	messageInvalid     = "0198f7c4-7b30-7f11-8a44-777777777777"
	messageUnsupported = "0198f7c4-7b30-7f11-8a44-888888888888"
	messageMalformed   = "0198f7c4-7b30-7f11-8a44-999999999999"
)

func TestReconcilerDuplicateSkippedStaleConflictAndInvalidTransitions(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	publisher := &capturePublisher{}
	applier := &scriptedApplier{}
	reconciler, err := New(store, edgeDeviceID, publisher, applier)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	reconciler.now = func() time.Time { return now }

	first := desired(1, messageOne)
	if err := reconciler.DesiredState(ctx, first); err != nil {
		t.Fatal(err)
	}
	record := readState(t, store)
	if record.ConditionStatus != "converged" || record.ObservedRevision != 1 || record.LastGoodRevision != 1 || applier.calls != 1 {
		t.Fatalf("first state = %+v calls=%d", record, applier.calls)
	}
	stableObservedID := record.ObservedMessageID
	if err := reconciler.Acknowledgement(ctx, &devicev1.Acknowledgement{
		MessageId: stableObservedID, Kind: devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := reconciler.DesiredState(ctx, first); err != nil {
		t.Fatal(err)
	}
	record = readState(t, store)
	if applier.calls != 1 || record.ObservedMessageID != stableObservedID || !record.ObservedPending {
		t.Fatalf("duplicate changed output: state=%+v calls=%d", record, applier.calls)
	}

	now = now.Add(time.Second)
	third := desired(3, messageThree)
	if err := reconciler.DesiredState(ctx, third); err != nil {
		t.Fatal(err)
	}
	record = readState(t, store)
	if record.LastGoodRevision != 3 || record.ConditionStatus != "converged" || applier.calls != 2 {
		t.Fatalf("skipped revision state = %+v calls=%d", record, applier.calls)
	}
	if err := reconciler.DesiredState(ctx, first); err != nil {
		t.Fatal(err)
	}
	if record = readState(t, store); record.DesiredRevision != 3 || record.LastGoodRevision != 3 || applier.calls != 2 {
		t.Fatalf("stale revision changed state = %+v calls=%d", record, applier.calls)
	}

	conflict := desired(3, messageConflict)
	conflict.Zones[0].DisplayName = "Conflicting"
	if err := reconciler.DesiredState(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	record = readState(t, store)
	if record.ConditionStatus != "failed" || record.ReasonCode != "revision_conflict" || record.LastGoodRevision != 3 {
		t.Fatalf("conflict state = %+v", record)
	}

	invalid := desired(4, messageInvalid)
	invalid.EdgeConfiguration.DeviceId = "0198f7c4-7b30-7f11-8a44-888888888888"
	if err := reconciler.DesiredState(ctx, invalid); err != nil {
		t.Fatal(err)
	}
	record = readState(t, store)
	if record.ConditionStatus != "failed" || record.ReasonCode != "identity_mismatch" || record.LastGoodRevision != 3 || record.AttemptCount != 1 {
		t.Fatalf("invalid state = %+v", record)
	}
}

func TestReconcilerRejectsMalformedAndUnsupportedWithoutReplacingLastGood(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	applier := &scriptedApplier{}
	reconciler, err := New(store, edgeDeviceID, &capturePublisher{}, applier)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.DesiredState(ctx, desired(1, messageOne)); err != nil {
		t.Fatal(err)
	}

	unsupported := desired(2, messageUnsupported)
	unsupported.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	if err := reconciler.DesiredState(ctx, unsupported); err != nil {
		t.Fatal(err)
	}
	record := readState(t, store)
	if record.ReasonCode != "unsupported_object" || record.LastGoodRevision != 1 || applier.calls != 1 {
		t.Fatalf("unsupported state = %+v calls=%d", record, applier.calls)
	}

	malformed := desired(3, messageMalformed)
	malformed.Zones[0].Cidr = "203.0.113.0/24"
	if err := reconciler.DesiredState(ctx, malformed); err != nil {
		t.Fatal(err)
	}
	record = readState(t, store)
	if record.ReasonCode != "invalid_snapshot" || record.LastGoodRevision != 1 || applier.calls != 1 {
		t.Fatalf("malformed state = %+v calls=%d", record, applier.calls)
	}
}

func TestReconcilerDisconnectRestartRepublishesStableObservedState(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	publisher := &capturePublisher{failures: 1}
	applier := &scriptedApplier{}
	reconciler, err := New(store, edgeDeviceID, publisher, applier)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.DesiredState(ctx, desired(1, messageOne)); err == nil {
		t.Fatal("disconnected publish unexpectedly succeeded")
	}
	record := readState(t, store)
	stableObservedID := record.ObservedMessageID
	if record.ConditionStatus != "converged" || !record.ObservedPending || stableObservedID == "" || applier.calls != 1 {
		t.Fatalf("disconnected state = %+v calls=%d", record, applier.calls)
	}

	reconnectedPublisher := &capturePublisher{}
	restartedApplier := &scriptedApplier{}
	restarted, err := New(store, edgeDeviceID, reconnectedPublisher, restartedApplier)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Trigger(ctx); err != nil {
		t.Fatal(err)
	}
	if len(reconnectedPublisher.observed) != 1 || reconnectedPublisher.observed[0].MessageId != stableObservedID || restartedApplier.calls != 0 {
		t.Fatalf("reconnected output = %+v calls=%d", reconnectedPublisher.observed, restartedApplier.calls)
	}
	if err := restarted.Acknowledgement(ctx, &devicev1.Acknowledgement{
		MessageId: stableObservedID, Kind: devicev1.AcknowledgementKind_ACKNOWLEDGEMENT_KIND_OBSERVED_STATE, Revision: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if record = readState(t, store); record.ObservedPending {
		t.Fatalf("acknowledged output remained pending: %+v", record)
	}
}

func TestReconcilerRetryMetadataSurvivesRestartAndExhausts(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	publisher := &capturePublisher{}
	applier := &scriptedApplier{retryFailures: 6}
	now := time.Date(2026, 8, 28, 13, 0, 0, 0, time.UTC)
	reconciler, err := New(store, edgeDeviceID, publisher, applier)
	if err != nil {
		t.Fatal(err)
	}
	reconciler.now = func() time.Time { return now }
	if err := reconciler.DesiredState(ctx, desired(1, messageOne)); err != nil {
		t.Fatal(err)
	}
	record := readState(t, store)
	if record.ConditionStatus != "retrying" || record.AttemptCount != 1 || record.RetryAt == nil || record.RetryAt.Sub(now) != time.Second {
		t.Fatalf("first retry state = %+v", record)
	}

	restarted, err := New(store, edgeDeviceID, publisher, applier)
	if err != nil {
		t.Fatal(err)
	}
	expectedDelays := []time.Duration{0, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	for expectedAttempt := uint32(2); expectedAttempt <= 6; expectedAttempt++ {
		now = *record.RetryAt
		restarted.now = func() time.Time { return now }
		if err := restarted.Trigger(ctx); err != nil {
			t.Fatal(err)
		}
		record = readState(t, store)
		if record.AttemptCount != expectedAttempt {
			t.Fatalf("attempt %d state = %+v", expectedAttempt, record)
		}
		if expectedAttempt < 6 && (record.ConditionStatus != "retrying" || record.RetryAt == nil || record.RetryAt.Sub(now) != expectedDelays[expectedAttempt]) {
			t.Fatalf("retry %d state = %+v want delay=%s", expectedAttempt, record, expectedDelays[expectedAttempt])
		}
	}
	if record.ConditionStatus != "failed" || record.ReasonCode != "retry_exhausted" || record.LastGoodRevision != 0 {
		t.Fatalf("exhausted state = %+v", record)
	}
}

func desired(revision uint64, messageID string) *devicev1.DesiredStateSnapshot {
	return &devicev1.DesiredStateSnapshot{
		MessageId: messageID, Revision: revision,
		EdgeConfiguration: &devicev1.EdgeConfiguration{DeviceId: edgeDeviceID, EnvironmentId: edgeEnvironmentID},
		Zones: []*devicev1.NetworkZoneMetadata{{
			ZoneId: edgeZoneID, DisplayName: "Primary", Cidr: "10.20.0.0/24", SourceRevision: 1,
		}},
		Decoys: []*devicev1.DecoyDesiredObject{},
	}
}

func edgeDecoy() *devicev1.DecoyDesiredObject {
	return &devicev1.DecoyDesiredObject{
		DecoyId: edgeDecoyID, ZoneId: edgeZoneID, DisplayName: "Finance file server",
		Family:           devicev1.DecoyFamily_DECOY_FAMILY_SMB,
		Persona:          devicev1.DecoyPersona_DECOY_PERSONA_WINDOWS_FILE_SERVICE_HOST,
		InteractionLevel: devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_LOW,
		Address:          "10.20.0.40", Pack: "smb-fileshare", PackVersion: "0.1.0",
		DesiredState:   devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_DEPLOYED,
		SourceRevision: 2,
	}
}

func TestValidateSnapshotAcceptsACanonicalDecoy(t *testing.T) {
	snapshot := desired(3, messageOne)
	snapshot.Decoys = []*devicev1.DecoyDesiredObject{edgeDecoy()}
	if reason := validateSnapshot(snapshot, edgeDeviceID); reason != "" {
		t.Fatalf("validateSnapshot() = %q", reason)
	}
}

// A decoy placed outside the zone it names must not be applied. The Control
// Plane rejects it too; the Edge repeats the check rather than trusting it.
func TestValidateSnapshotRejectsADecoyOutsideItsZone(t *testing.T) {
	snapshot := desired(3, messageOne)
	decoy := edgeDecoy()
	decoy.Address = "10.21.0.40"
	snapshot.Decoys = []*devicev1.DecoyDesiredObject{decoy}
	if reason := validateSnapshot(snapshot, edgeDeviceID); reason != "invalid_snapshot" {
		t.Fatalf("out-of-zone decoy reason = %q", reason)
	}
}

// An unrecognised vocabulary is refused with its own reason code, so an
// operator can tell "this Edge does not understand this decoy" apart from
// "this snapshot is malformed".
func TestValidateSnapshotRejectsAnUnsupportedDecoyVocabulary(t *testing.T) {
	cases := map[string]func(*devicev1.DecoyDesiredObject){
		"unspecified family": func(d *devicev1.DecoyDesiredObject) {
			d.Family = devicev1.DecoyFamily_DECOY_FAMILY_UNSPECIFIED
		},
		"unspecified persona": func(d *devicev1.DecoyDesiredObject) {
			d.Persona = devicev1.DecoyPersona_DECOY_PERSONA_UNSPECIFIED
		},
		"interaction level disagrees with family": func(d *devicev1.DecoyDesiredObject) {
			d.InteractionLevel = devicev1.DecoyInteractionLevel_DECOY_INTERACTION_LEVEL_MEDIUM
		},
		"unspecified lifecycle": func(d *devicev1.DecoyDesiredObject) {
			d.DesiredState = devicev1.DecoyDesiredLifecycle_DECOY_DESIRED_LIFECYCLE_UNSPECIFIED
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := desired(3, messageOne)
			decoy := edgeDecoy()
			mutate(decoy)
			snapshot.Decoys = []*devicev1.DecoyDesiredObject{decoy}
			if reason := validateSnapshot(snapshot, edgeDeviceID); reason != "unsupported_object" {
				t.Fatalf("validateSnapshot() = %q, want unsupported_object", reason)
			}
		})
	}
}

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	root := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{
		DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func readState(t *testing.T, store *storage.Store) storage.ReconciliationRecord {
	t.Helper()
	record, err := store.ReconciliationState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return record
}

type scriptedApplier struct {
	calls         int
	retryFailures int
}

func (applier *scriptedApplier) Apply(context.Context, *devicev1.DesiredStateSnapshot) error {
	applier.calls++
	if applier.calls <= applier.retryFailures {
		return &ApplyFailure{ReasonCode: "apply_unavailable", Retryable: true}
	}
	return nil
}

type capturePublisher struct {
	mu       sync.Mutex
	observed []*devicev1.ObservedState
	failures int
}

func (publisher *capturePublisher) PublishObserved(observed *devicev1.ObservedState) error {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.failures > 0 {
		publisher.failures--
		return errors.New("device channel unavailable")
	}
	publisher.observed = append(publisher.observed, observed)
	return nil
}
