package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const (
	testDeviceID      = "0198f7c4-7b30-7f11-8a44-111111111111"
	testEnvironmentID = "0198f7c4-7b30-7f11-8a44-222222222222"
	testMessageID     = "0198f7c4-7b30-7f11-8a44-333333333333"
	testZoneID        = "0198f7c4-7b30-7f11-8a44-444444444444"
	testDecoyID       = "0198f7c4-7b30-7f11-8a44-666666666666"
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
		Decoys:            []Decoy{},
	}
}

func validDecoy() Decoy {
	return Decoy{
		DecoyID: testDecoyID, ZoneID: testZoneID, DisplayName: "Finance file server",
		Family: "smb", Persona: "windows_file_service_host", InteractionLevel: "low",
		Address: "10.20.0.40", Pack: "smb-fileshare", PackVersion: "0.1.0",
		DesiredState: "deployed", SourceRevision: 1,
	}
}

func TestValidateSnapshotAcceptsACanonicalDecoy(t *testing.T) {
	snapshot := validSnapshot()
	snapshot.Decoys = []Decoy{validDecoy()}
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("ValidateSnapshot() = %v", err)
	}
}

// A decoy the Control Plane places outside the zone it names must never reach
// an Edge: that is how a decoy ends up answering on a production address.
func TestValidateSnapshotRejectsADecoyOutsideItsZone(t *testing.T) {
	snapshot := validSnapshot()
	decoy := validDecoy()
	decoy.Address = "10.21.0.40"
	snapshot.Decoys = []Decoy{decoy}
	if err := ValidateSnapshot(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("out-of-zone decoy error = %v", err)
	}
}

func TestValidateSnapshotRejectsVocabularyAndPairingViolations(t *testing.T) {
	cases := map[string]func(*Decoy){
		"unknown family":        func(d *Decoy) { d.Family = "telnet" },
		"unknown persona":       func(d *Decoy) { d.Persona = "chief_executive" },
		"unknown interaction":   func(d *Decoy) { d.InteractionLevel = "high" },
		"ssh must be medium":    func(d *Decoy) { d.Family, d.Persona = "ssh", "linux_admin_server" },
		"smb must be low":       func(d *Decoy) { d.InteractionLevel = "medium" },
		"removed never travels": func(d *Decoy) { d.DesiredState = "removed" },
		"unknown desired state": func(d *Decoy) { d.DesiredState = "paused" },
		"pack name shape":       func(d *Decoy) { d.Pack = "SMB Fileshare" },
		"pack version shape":    func(d *Decoy) { d.PackVersion = "0.1" },
		"digest shape":          func(d *Decoy) { d.PackDigest = "sha256:zz" },
		"zero source revision":  func(d *Decoy) { d.SourceRevision = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			snapshot := validSnapshot()
			decoy := validDecoy()
			mutate(&decoy)
			snapshot.Decoys = []Decoy{decoy}
			if err := ValidateSnapshot(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("ValidateSnapshot() = %v", err)
			}
		})
	}
}

// An empty digest is correct, not a defect: P2-W4 has not defined a manifest to
// hash, and inventing one would assert an unverified artifact identity.
func TestValidateSnapshotAcceptsAnAbsentPackDigest(t *testing.T) {
	snapshot := validSnapshot()
	decoy := validDecoy()
	decoy.PackDigest = ""
	snapshot.Decoys = []Decoy{decoy}
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("ValidateSnapshot() = %v", err)
	}
}

// The 64-decoy bound is the device-channel message-size guarantee P1-W6 made.
func TestValidateSnapshotHoldsTheDecoyBound(t *testing.T) {
	snapshot := validSnapshot()
	for index := 0; index <= MaxDecoys; index++ {
		decoy := validDecoy()
		decoy.DecoyID = decoyIDAt(index)
		decoy.Address = addressAt(index)
		snapshot.Decoys = append(snapshot.Decoys, decoy)
	}
	if err := ValidateSnapshot(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("over-bound snapshot error = %v", err)
	}
	snapshot.Decoys = snapshot.Decoys[:MaxDecoys]
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("at-bound snapshot error = %v", err)
	}
	payload, err := MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 64*1024 {
		t.Fatalf("64 decoys encode to %d bytes, beyond the device-channel budget", len(payload))
	}
}

func decoyIDAt(index int) string {
	return fmt.Sprintf("0192f0a0-0000-7000-8000-%012d", index)
}

func addressAt(index int) string {
	return fmt.Sprintf("10.20.0.%d", index+1)
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
