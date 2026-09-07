package deception

import (
	"context"
	"errors"
	"testing"
	"time"
)

type repositoryStub struct {
	created      Write
	createCalls  int
	transitions  []DesiredState
	removeCalls  int
	reportCalls  int
	lastMutation Mutation
	view         View
}

func (r *repositoryStub) ListDecoys(context.Context, string, int32) ([]View, error) {
	return []View{r.view}, nil
}

func (r *repositoryStub) Decoy(context.Context, string, string) (View, error) { return r.view, nil }

func (r *repositoryStub) CreateDecoy(_ context.Context, _ string, write Write, mutation Mutation) (Decoy, error) {
	r.createCalls++
	r.created = write
	r.lastMutation = mutation
	return Decoy{DecoyID: testDecoyID, Revision: 1}, nil
}

func (r *repositoryStub) UpdateDecoy(_ context.Context, _, _ string, write Write, _ int64, _ Mutation) (Decoy, error) {
	r.created = write
	return Decoy{DecoyID: testDecoyID, Revision: 2}, nil
}

func (r *repositoryStub) SetDecoyDesiredState(
	_ context.Context, _, _ string, state DesiredState, _ int64, _ Mutation,
) (Decoy, error) {
	r.transitions = append(r.transitions, state)
	return Decoy{DecoyID: testDecoyID, DesiredState: state, Revision: 3}, nil
}

func (r *repositoryStub) RemoveDecoy(_ context.Context, _, _ string, _ int64, _ Mutation) (Decoy, error) {
	r.removeCalls++
	return Decoy{DecoyID: testDecoyID, DesiredState: DesiredRemoved, Revision: 4}, nil
}

func (r *repositoryStub) RecordObservations(context.Context, Report) error {
	r.reportCalls++
	return nil
}

const testEnvironmentID = "0198f7c4-7b30-7f11-8a44-222222222222"
const testZoneID = "0198f7c4-7b30-7f11-8a44-444444444444"

func validInput() Input {
	return Input{
		ZoneID: testZoneID, DisplayName: "  Finance file server  ",
		Family: "smb", Persona: "windows_file_service_host",
		Address: "10.20.0.40", Pack: "smb-fileshare", PackVersion: "0.1.0",
	}
}

func newTestService(t *testing.T) (*Service, *repositoryStub) {
	t.Helper()
	repository := &repositoryStub{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository
}

// Interaction level and digest are resolved from the server-side index, not
// supplied. There is nowhere in the request for a runtime detail to enter.
func TestCreateResolvesInteractionLevelAndDigestFromThePackIndex(t *testing.T) {
	service, repository := newTestService(t)
	if _, err := service.CreateDecoy(
		context.Background(), testEnvironmentID, validInput(), Mutation{ActorID: "owner"},
	); err != nil {
		t.Fatal(err)
	}
	if repository.createCalls != 1 {
		t.Fatalf("CreateDecoy calls = %d", repository.createCalls)
	}
	write := repository.created
	if write.InteractionLevel != InteractionLow {
		t.Fatalf("interaction level = %q, want the pack index's value", write.InteractionLevel)
	}
	if write.PackDigest != "" {
		t.Fatalf("pack digest = %q; no manifest exists to hash yet", write.PackDigest)
	}
	if write.Name.DisplayName != "Finance file server" {
		t.Fatalf("display name = %q, want the trimmed NFC form", write.Name.DisplayName)
	}
	if repository.lastMutation.OccurredAt.IsZero() {
		t.Fatal("mutation was not stamped with an occurrence time")
	}
}

func TestCreateRejectsInputOutsideTheClosedVocabulariesAndIndex(t *testing.T) {
	cases := map[string]func(*Input){
		"unknown family":        func(i *Input) { i.Family = "telnet" },
		"unknown persona":       func(i *Input) { i.Persona = "chief_executive" },
		"family disagrees":      func(i *Input) { i.Family = "ssh" },
		"unknown pack":          func(i *Input) { i.Pack = "does-not-exist" },
		"unknown version":       func(i *Input) { i.PackVersion = "9.9.9" },
		"public address":        func(i *Input) { i.Address = "203.0.113.5" },
		"empty display name":    func(i *Input) { i.DisplayName = "   " },
		"control character":     func(i *Input) { i.DisplayName = "Finance\x07server" },
		"malformed zone":        func(i *Input) { i.ZoneID = "not-a-uuid" },
		"prefix not an address": func(i *Input) { i.Address = "10.20.0.0/24" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			service, repository := newTestService(t)
			input := validInput()
			mutate(&input)
			_, err := service.CreateDecoy(
				context.Background(), testEnvironmentID, input, Mutation{ActorID: "owner"},
			)
			if err == nil {
				t.Fatal("CreateDecoy accepted invalid input")
			}
			if repository.createCalls != 0 {
				t.Fatalf("invalid input reached storage: %d calls", repository.createCalls)
			}
		})
	}
}

// Enable and disable are separate operations because WC-D16 assigns
// confirmation levels to operations, not to fields inside one.
func TestEnableAndDisableAreDistinctTransitions(t *testing.T) {
	service, repository := newTestService(t)
	ctx := context.Background()
	mutation := Mutation{ActorID: "owner"}
	if _, err := service.EnableDecoy(ctx, testEnvironmentID, testDecoyID, 1, mutation); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DisableDecoy(ctx, testEnvironmentID, testDecoyID, 2, mutation); err != nil {
		t.Fatal(err)
	}
	want := []DesiredState{DesiredDeployed, DesiredDisabled}
	if len(repository.transitions) != len(want) {
		t.Fatalf("transitions = %v", repository.transitions)
	}
	for index, state := range want {
		if repository.transitions[index] != state {
			t.Fatalf("transition %d = %q, want %q", index, repository.transitions[index], state)
		}
	}
}

func TestMutationsRequireAnActorAndAPositiveRevision(t *testing.T) {
	service, repository := newTestService(t)
	ctx := context.Background()
	if _, err := service.CreateDecoy(ctx, testEnvironmentID, validInput(), Mutation{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("actorless create error = %v", err)
	}
	if _, err := service.EnableDecoy(ctx, testEnvironmentID, testDecoyID, 0, Mutation{ActorID: "owner"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("zero-revision enable error = %v", err)
	}
	if err := service.RemoveDecoy(ctx, testEnvironmentID, "not-a-uuid", 1, Mutation{ActorID: "owner"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("malformed identity error = %v", err)
	}
	if repository.createCalls != 0 || repository.removeCalls != 0 || len(repository.transitions) != 0 {
		t.Fatal("an invalid mutation reached storage")
	}
}

// The service validates an Edge report before storage sees it, so a malformed
// claim never becomes stored truth.
func TestRecordObservationsValidatesBeforeStorage(t *testing.T) {
	service, repository := newTestService(t)
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	valid := Report{
		ReportID: testReportID, ObservedAt: observedAt, DeviceID: testDeviceID,
		Observations: []Observation{observationAt(observedAt)},
	}
	if err := service.RecordObservations(context.Background(), valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.ReportID = "not-a-uuid"
	if err := service.RecordObservations(context.Background(), invalid); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid report error = %v", err)
	}
	if repository.reportCalls != 1 {
		t.Fatalf("RecordObservations reached storage %d times, want 1", repository.reportCalls)
	}
}
