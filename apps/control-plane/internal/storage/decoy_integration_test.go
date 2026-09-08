//go:build integration

package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/reconciliation"
)

type decoyFixture struct {
	store         *Store
	service       *deception.Service
	environmentID string
	zoneID        string
	deviceID      string
}

func newDecoyFixture(t *testing.T) decoyFixture {
	t.Helper()
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 12)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)

	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	mutation := environment.Mutation{ActorID: "owner-1"}
	env, err := environmentService.CreateEnvironment(ctx, "Production", mutation)
	if err != nil {
		t.Fatal(err)
	}
	zone, err := environmentService.CreateZone(ctx, env.EnvironmentID, "Servers", "10.20.0.0/24", mutation)
	if err != nil {
		t.Fatal(err)
	}
	var deviceID string
	if err := store.pool.QueryRow(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES (uuidv7(), $1, 'edge-1', 'active', clock_timestamp(), clock_timestamp())
RETURNING device_id::text`, env.EnvironmentID).Scan(&deviceID); err != nil {
		t.Fatal(err)
	}
	service, err := deception.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	return decoyFixture{
		store: store, service: service,
		environmentID: env.EnvironmentID, zoneID: zone.ZoneID, deviceID: deviceID,
	}
}

func (f decoyFixture) input(name, address string) deception.Input {
	return deception.Input{
		ZoneID: f.zoneID, DisplayName: name, Family: "smb",
		Persona: "windows_file_service_host", Address: address,
		Pack: "smb-fileshare", PackVersion: "0.1.0",
	}
}

func TestDecoyLifecycleObservedStateAndAudit(t *testing.T) {
	ctx := context.Background()
	fixture := newDecoyFixture(t)
	mutation := deception.Mutation{ActorID: "owner-1", RequestID: "request-9"}

	decoy, err := fixture.service.CreateDecoy(ctx, fixture.environmentID, fixture.input("Finance file server", "10.20.0.40"), mutation)
	if err != nil {
		t.Fatal(err)
	}
	if decoy.InteractionLevel != deception.InteractionLow || decoy.PackDigest != "" {
		t.Fatalf("created decoy = %+v", decoy)
	}

	// 10.1.3: a decoy with no observed report reads unknown, never healthy or
	// absent. This is the package's load-bearing guarantee.
	view, err := fixture.service.Decoy(ctx, fixture.environmentID, decoy.DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Observed.State != deception.ObservedUnknown {
		t.Fatalf("unreported decoy observed state = %q", view.Observed.State)
	}
	for _, condition := range view.Observed.Conditions {
		if condition.Status != deception.StatusUnknown {
			t.Fatalf("unreported condition %q = %q", condition.Type, condition.Status)
		}
	}

	// 10.1.1: a decoy address outside its zone CIDR is rejected.
	if _, err := fixture.service.CreateDecoy(
		ctx, fixture.environmentID, fixture.input("Outside", "10.21.0.40"), mutation,
	); !errors.Is(err, deception.ErrAddressOutsideZone) {
		t.Fatalf("out-of-zone create error = %v", err)
	}

	// Address and name conflicts are distinguishable.
	if _, err := fixture.service.CreateDecoy(
		ctx, fixture.environmentID, fixture.input("Another name", "10.20.0.40"), mutation,
	); !errors.Is(err, deception.ErrAddressConflict) {
		t.Fatalf("address conflict error = %v", err)
	}
	if _, err := fixture.service.CreateDecoy(
		ctx, fixture.environmentID, fixture.input("finance FILE server", "10.20.0.41"), mutation,
	); !errors.Is(err, deception.ErrNameConflict) {
		t.Fatalf("name conflict error = %v", err)
	}

	// 10.1.6: If-Match conflicts behave as the zone domain's do.
	if _, err := fixture.service.DisableDecoy(
		ctx, fixture.environmentID, decoy.DecoyID, decoy.Revision+7, mutation,
	); !errors.Is(err, deception.ErrPreconditionFailed) {
		t.Fatalf("stale revision error = %v", err)
	}

	disabled, err := fixture.service.DisableDecoy(ctx, fixture.environmentID, decoy.DecoyID, decoy.Revision, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.DesiredState != deception.DesiredDisabled || disabled.Revision != decoy.Revision+1 {
		t.Fatalf("disabled decoy = %+v", disabled)
	}
	enabled, err := fixture.service.EnableDecoy(ctx, fixture.environmentID, decoy.DecoyID, disabled.Revision, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if enabled.DesiredState != deception.DesiredDeployed {
		t.Fatalf("enabled decoy = %+v", enabled)
	}

	// 10.1.4: observed state is separately addressable and is written only by
	// the device channel.
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	interaction := observedAt.Add(-time.Minute)
	observation := deception.UnreportedObservation(decoy.DecoyID, observedAt)
	observation.State = deception.ObservedDeployed
	observation.LastInteractionAt = &interaction
	observation.Conditions[0].Status = deception.StatusTrue
	observation.Conditions[0].Reason = "running"
	report := deception.Report{
		ReportID: newTestUUIDv7(t, fixture.store), ObservedAt: observedAt,
		DeviceID: fixture.deviceID, Observations: []deception.Observation{observation},
	}
	if err := fixture.service.RecordObservations(ctx, report); err != nil {
		t.Fatal(err)
	}
	view, err = fixture.service.Decoy(ctx, fixture.environmentID, decoy.DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Observed.State != deception.ObservedDeployed {
		t.Fatalf("observed state = %q", view.Observed.State)
	}
	if view.Decoy.DesiredState != deception.DesiredDeployed {
		t.Fatalf("desired state = %q", view.Decoy.DesiredState)
	}
	if view.Observed.LastInteractionAt == nil || !view.Observed.LastInteractionAt.Equal(interaction) {
		t.Fatalf("last interaction = %v, want %v", view.Observed.LastInteractionAt, interaction)
	}
	if view.Observed.ReportingDeviceID != fixture.deviceID {
		t.Fatalf("reporting device = %q", view.Observed.ReportingDeviceID)
	}
	if view.Observed.Conditions[0].Status != deception.StatusTrue {
		t.Fatalf("reported condition = %+v", view.Observed.Conditions[0])
	}
	// A dimension the Edge reported as Unknown stays Unknown.
	if view.Observed.Conditions[1].Status != deception.StatusUnknown {
		t.Fatalf("unreported condition = %+v", view.Observed.Conditions[1])
	}

	// 10.1.5: a revoked device's decoys report unmanaged and remain listed.
	if _, err := fixture.store.pool.Exec(ctx, `
UPDATE guardian_devices.devices SET state = 'revoked', updated_at = clock_timestamp()
WHERE device_id = $1`, fixture.deviceID); err != nil {
		t.Fatal(err)
	}
	views, err := fixture.service.ListDecoys(ctx, fixture.environmentID, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("listed %d decoys; a decoy Guardian cannot manage must stay visible", len(views))
	}
	if views[0].Observed.State != deception.ObservedUnmanaged {
		t.Fatalf("revoked device's decoy observed state = %q, want unmanaged", views[0].Observed.State)
	}
	// SEC-06 keeps the last-known conditions: the decoy may still be running.
	if views[0].Observed.Conditions[0].Status != deception.StatusTrue {
		t.Fatalf("unmanaged decoy lost its last-known condition: %+v", views[0].Observed.Conditions[0])
	}
	if _, err := fixture.store.pool.Exec(ctx, `
UPDATE guardian_devices.devices SET state = 'active', updated_at = clock_timestamp()
WHERE device_id = $1`, fixture.deviceID); err != nil {
		t.Fatal(err)
	}

	// 10.1.10: removing a decoy leaves its audit trail intact.
	current, err := fixture.service.Decoy(ctx, fixture.environmentID, decoy.DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RemoveDecoy(
		ctx, fixture.environmentID, decoy.DecoyID, current.Decoy.Revision, mutation,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Decoy(ctx, fixture.environmentID, decoy.DecoyID); !errors.Is(err, deception.ErrNotFound) {
		t.Fatalf("removed decoy still readable: %v", err)
	}

	// 10.1.7: every lifecycle transition emitted its audit event and the
	// vocabulary migration accepted each new pair.
	rows, err := fixture.store.pool.Query(ctx, `
SELECT action FROM guardian_audit.records
WHERE object_type = 'decoy' AND object_id = $1
ORDER BY sequence`, decoy.DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	want := []string{
		string(audit.ActionDecoyCreated),
		string(audit.ActionDecoyDisabled),
		string(audit.ActionDecoyEnabled),
		string(audit.ActionDecoyRemoved),
	}
	if len(actions) != len(want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
	for index, action := range want {
		if actions[index] != action {
			t.Fatalf("audit action %d = %q, want %q", index, actions[index], action)
		}
	}

	// The address released by removal is reusable, and the new decoy is a
	// distinct identity: removal retires a decoy, it does not erase one.
	replacement, err := fixture.service.CreateDecoy(
		ctx, fixture.environmentID, fixture.input("Finance file server", "10.20.0.40"), mutation,
	)
	if err != nil {
		t.Fatalf("address was not released by removal: %v", err)
	}
	if replacement.DecoyID == decoy.DecoyID {
		t.Fatal("replacement reused the removed decoy's identity")
	}
}

// A device may report only on decoys its own environment owns.
func TestDecoyObservationsAreScopedToTheReportingDeviceEnvironment(t *testing.T) {
	ctx := context.Background()
	fixture := newDecoyFixture(t)
	mutation := deception.Mutation{ActorID: "owner-1"}
	decoy, err := fixture.service.CreateDecoy(ctx, fixture.environmentID, fixture.input("Finance", "10.20.0.40"), mutation)
	if err != nil {
		t.Fatal(err)
	}

	environmentService, err := environment.NewService(fixture.store)
	if err != nil {
		t.Fatal(err)
	}
	other, err := environmentService.CreateEnvironment(ctx, "Staging", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	var foreignDevice string
	if err := fixture.store.pool.QueryRow(ctx, `
INSERT INTO guardian_devices.devices (device_id, environment_id, display_name, state, created_at, updated_at)
VALUES (uuidv7(), $1, 'edge-2', 'active', clock_timestamp(), clock_timestamp())
RETURNING device_id::text`, other.EnvironmentID).Scan(&foreignDevice); err != nil {
		t.Fatal(err)
	}

	observedAt := time.Now().UTC()
	observation := deception.UnreportedObservation(decoy.DecoyID, observedAt)
	observation.State = deception.ObservedDeployed
	err = fixture.service.RecordObservations(ctx, deception.Report{
		ReportID: newTestUUIDv7(t, fixture.store), ObservedAt: observedAt,
		DeviceID: foreignDevice, Observations: []deception.Observation{observation},
	})
	if !errors.Is(err, deception.ErrNotFound) {
		t.Fatalf("a foreign device reported on another environment's decoy: %v", err)
	}
	view, err := fixture.service.Decoy(ctx, fixture.environmentID, decoy.DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Observed.State != deception.ObservedUnknown {
		t.Fatalf("rejected report changed observed state to %q", view.Observed.State)
	}
}

// 10.1.8: the desired-state snapshot carries decoys and stays inside the
// device-channel size bound at the 64-decoy limit.
func TestDesiredStateSnapshotCarriesDecoysWithinTheChannelBound(t *testing.T) {
	ctx := context.Background()
	fixture := newDecoyFixture(t)
	mutation := deception.Mutation{ActorID: "owner-1"}
	for index := 0; index < reconciliation.MaxDecoys; index++ {
		address := "10.20.0." + itoa(index+1)
		name := "Decoy " + itoa(index+1)
		if _, err := fixture.service.CreateDecoy(ctx, fixture.environmentID, fixture.input(name, address), mutation); err != nil {
			t.Fatalf("create decoy %d: %v", index, err)
		}
	}
	// The 65th is refused at the moment it is asked for, rather than silently
	// dropped from the snapshot later.
	if _, err := fixture.service.CreateDecoy(
		ctx, fixture.environmentID, fixture.input("One too many", "10.20.0.200"), mutation,
	); !errors.Is(err, deception.ErrDecoyBudgetExhausted) {
		t.Fatalf("over-budget create error = %v", err)
	}
	snapshot, err := fixture.store.EnsureCurrent(ctx, fixture.deviceID, "device-channel")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Decoys) != reconciliation.MaxDecoys {
		t.Fatalf("snapshot carries %d decoys, want %d", len(snapshot.Decoys), reconciliation.MaxDecoys)
	}
	if err := reconciliation.ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("snapshot rejected by its own validator: %v", err)
	}
	payload, err := reconciliation.MarshalSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) > 1<<20 {
		t.Fatalf("64-decoy snapshot encodes to %d bytes, beyond the stored payload bound", len(payload))
	}

	// A removed decoy leaves the snapshot rather than travelling as a tombstone.
	view, err := fixture.service.Decoy(ctx, fixture.environmentID, snapshot.Decoys[0].DecoyID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.service.RemoveDecoy(
		ctx, fixture.environmentID, view.Decoy.DecoyID, view.Decoy.Revision, mutation,
	); err != nil {
		t.Fatal(err)
	}
	next, err := fixture.store.EnsureCurrent(ctx, fixture.deviceID, "device-channel")
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Decoys) != reconciliation.MaxDecoys-1 {
		t.Fatalf("snapshot after removal carries %d decoys", len(next.Decoys))
	}
	for _, entry := range next.Decoys {
		if entry.DecoyID == view.Decoy.DecoyID {
			t.Fatal("a removed decoy travelled to the Edge")
		}
		if entry.DesiredState == "removed" {
			t.Fatal("a removed lifecycle value reached the wire")
		}
	}
	if next.Revision <= snapshot.Revision {
		t.Fatalf("removal did not advance the desired revision: %d then %d", snapshot.Revision, next.Revision)
	}
}

func newTestUUIDv7(t *testing.T, store *Store) string {
	t.Helper()
	var value string
	if err := store.pool.QueryRow(context.Background(), "SELECT uuidv7()::text").Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
