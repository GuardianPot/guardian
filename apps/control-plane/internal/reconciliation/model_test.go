package reconciliation

import (
	"context"
	"errors"
	"testing"
	"time"
)

const (
	testDeviceID      = "0198f7c4-7b30-7f11-8a44-111111111111"
	testEnvironmentID = "0198f7c4-7b30-7f11-8a44-222222222222"
	testMessageID     = "0198f7c4-7b30-7f11-8a44-333333333333"
	testZoneID        = "0198f7c4-7b30-7f11-8a44-444444444444"
)

func TestSnapshotValidationAndContentDigestAreDeterministic(t *testing.T) {
	snapshot := validSnapshot()
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	digest, err := ContentDigest(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	otherEnvelope := snapshot
	otherEnvelope.MessageID = "0198f7c4-7b30-7f11-8a44-555555555555"
	otherEnvelope.Revision = 9
	otherDigest, err := ContentDigest(otherEnvelope)
	if err != nil || digest != otherDigest {
		t.Fatalf("content digest changed with transport envelope: %x %x %v", digest, otherDigest, err)
	}
	payload, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSnapshot(payload)
	if err != nil || parsed.MessageID != snapshot.MessageID || len(parsed.Zones) != 1 {
		t.Fatalf("round trip = (%+v, %v)", parsed, err)
	}
	if _, err := ParseSnapshot(append(payload, []byte(` {}`)...)); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("trailing JSON value was accepted: %v", err)
	}

	invalid := snapshot
	invalid.Zones = append(invalid.Zones, invalid.Zones[0])
	if !errors.Is(ValidateSnapshot(invalid), ErrInvalidSnapshot) {
		t.Fatal("duplicate/non-ordered zone was accepted")
	}
	invalid = snapshot
	invalid.Zones[0].CIDR = "203.0.113.0/24"
	if !errors.Is(ValidateSnapshot(invalid), ErrInvalidSnapshot) {
		t.Fatal("public zone was accepted")
	}
}

func TestObservedValidationRequiresTruthfulConvergenceAndRetry(t *testing.T) {
	now := time.Now().UTC()
	observed := ObservedState{
		MessageID: testMessageID, DesiredRevision: 4, ObservedRevision: 4, LastGoodRevision: 4,
		Condition: Condition{Status: ConditionConverged, ReasonCode: "applied", AttemptCount: 1, LastTransitionTime: now},
	}
	if err := ValidateObserved(observed); err != nil {
		t.Fatal(err)
	}
	invalid := observed
	invalid.ObservedRevision = 3
	if !errors.Is(ValidateObserved(invalid), ErrInvalidObserved) {
		t.Fatal("false convergence was accepted")
	}
	retryAt := now.Add(time.Second)
	observed.Condition = Condition{Status: ConditionRetrying, ReasonCode: "apply_failed", AttemptCount: 2, RetryAt: &retryAt, LastTransitionTime: now}
	observed.ObservedRevision = 3
	observed.LastGoodRevision = 3
	if err := ValidateObserved(observed); err != nil {
		t.Fatal(err)
	}
}

func TestServiceRejectsIdentityAndDelegatesDurableOperations(t *testing.T) {
	repository := &repositoryStub{snapshot: validSnapshot()}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.DesiredState(context.Background(), "not-a-device"); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("invalid identity error = %v", err)
	}
	if _, err := service.DesiredState(context.Background(), testDeviceID); err != nil || repository.ensureCalls != 1 {
		t.Fatalf("DesiredState() = (%v, calls=%d)", err, repository.ensureCalls)
	}
}

func validSnapshot() Snapshot {
	return Snapshot{
		MessageID: testMessageID, Revision: 1,
		EdgeConfiguration: EdgeConfiguration{DeviceID: testDeviceID, EnvironmentID: testEnvironmentID},
		Zones:             []Zone{{ZoneID: testZoneID, DisplayName: "Primary", CIDR: "10.20.0.0/24", SourceRevision: 1}},
		PlaceholderDecoys: []PlaceholderDecoy{},
	}
}

type repositoryStub struct {
	snapshot    Snapshot
	ensureCalls int
}

func (repository *repositoryStub) EnsureCurrent(context.Context, string, string) (Snapshot, error) {
	repository.ensureCalls++
	return repository.snapshot, nil
}
func (*repositoryStub) RecordObserved(context.Context, string, ObservedState) error { return nil }
func (*repositoryStub) AcknowledgeDesired(context.Context, string, Acknowledgement) error {
	return nil
}
