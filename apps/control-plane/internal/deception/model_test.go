package deception

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	testDecoyID  = "0198f7c4-7b30-7f11-8a44-666666666666"
	testDeviceID = "0198f7c4-7b30-7f11-8a44-111111111111"
	testReportID = "0198f7c4-7b30-7f11-8a44-777777777777"
)

// The most important test in the package. A decoy nothing has reported on is
// unknown, and every dimension of it is Unknown. If this ever reads deployed,
// healthy, or absent, Guardian is claiming coverage it has not confirmed.
func TestUnreportedDecoyIsUnknownAndNeverHealthyOrAbsent(t *testing.T) {
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	observation := UnreportedObservation(testDecoyID, observedAt)
	if observation.State != ObservedUnknown {
		t.Fatalf("unreported decoy state = %q, want %q", observation.State, ObservedUnknown)
	}
	if observation.LastInteractionAt != nil || observation.ReportedAt != nil ||
		observation.ReportingDeviceID != "" || observation.DesiredRevision != nil {
		t.Fatalf("unreported decoy carried report metadata: %+v", observation)
	}
	types := ConditionTypes()
	if len(observation.Conditions) != len(types) {
		t.Fatalf("unreported decoy has %d conditions, want the complete set of %d",
			len(observation.Conditions), len(types))
	}
	for index, condition := range observation.Conditions {
		if condition.Type != types[index] {
			t.Fatalf("condition %d = %q, want %q", index, condition.Type, types[index])
		}
		if condition.Status != StatusUnknown {
			t.Fatalf("condition %q = %q; nothing has observed it", condition.Type, condition.Status)
		}
		if condition.Reason != "not_observed" {
			t.Fatalf("condition %q reason = %q", condition.Type, condition.Reason)
		}
	}
}

func TestClosedVocabulariesRejectAnythingOutsideThem(t *testing.T) {
	for _, family := range []Family{FamilySSH, FamilyHTTP, FamilyPostgres, FamilySMB} {
		if !family.Valid() {
			t.Errorf("approved family %q rejected", family)
		}
	}
	for _, family := range []Family{"", "telnet", "rdp", "SSH"} {
		if family.Valid() {
			t.Errorf("family %q accepted outside DC-01", family)
		}
	}
	for _, persona := range []Persona{
		PersonaLinuxAdminServer, PersonaInternalAdminWebApp,
		PersonaDatabaseServer, PersonaWindowsFileServiceHost,
	} {
		if !persona.Valid() {
			t.Errorf("approved persona %q rejected", persona)
		}
	}
	for _, persona := range []Persona{"", "chief_executive", "<script>"} {
		if persona.Valid() {
			t.Errorf("persona %q accepted outside DC-11", persona)
		}
	}
	for _, level := range []InteractionLevel{"", "high", "HIGH"} {
		if level.Valid() {
			t.Errorf("interaction level %q accepted; high interaction is out of the MVP", level)
		}
	}
}

// INT-01 pairs interaction level to family. It is a rule, not a preference.
func TestInteractionLevelPairingFollowsINT01(t *testing.T) {
	valid := []struct {
		family Family
		level  InteractionLevel
	}{
		{FamilySSH, InteractionMedium},
		{FamilyHTTP, InteractionLow},
		{FamilySMB, InteractionLow},
		{FamilyPostgres, InteractionLow},
		{FamilyPostgres, InteractionMedium},
	}
	for _, pair := range valid {
		if err := InteractionLevelFor(pair.family, pair.level); err != nil {
			t.Errorf("InteractionLevelFor(%q, %q) = %v", pair.family, pair.level, err)
		}
	}
	invalid := []struct {
		family Family
		level  InteractionLevel
	}{
		{FamilySSH, InteractionLow},
		{FamilyHTTP, InteractionMedium},
		{FamilySMB, InteractionMedium},
		{"telnet", InteractionLow},
	}
	for _, pair := range invalid {
		if err := InteractionLevelFor(pair.family, pair.level); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("InteractionLevelFor(%q, %q) = %v, want rejection", pair.family, pair.level, err)
		}
	}
}

// A decoy placed outside the zone it claims could answer on a production
// address. That is the failure this check exists to prevent.
func TestAddressWithinZoneRejectsPlacementOutsideTheZone(t *testing.T) {
	if err := AddressWithinZone("10.20.0.40", "10.20.0.0/24"); err != nil {
		t.Fatalf("in-zone address rejected: %v", err)
	}
	outside := []struct {
		address string
		cidr    string
	}{
		{"10.21.0.40", "10.20.0.0/24"},
		{"192.168.1.5", "10.20.0.0/24"},
		{"10.20.0.0", "10.20.0.0/24"},
		{"10.20.0.255", "10.20.0.0/24"},
	}
	for _, placement := range outside {
		if err := AddressWithinZone(placement.address, placement.cidr); !errors.Is(err, ErrAddressOutsideZone) {
			t.Errorf("AddressWithinZone(%q, %q) = %v, want rejection", placement.address, placement.cidr, err)
		}
	}
	// A /31 or /32 has no network or broadcast address to exclude.
	if err := AddressWithinZone("10.20.0.1", "10.20.0.0/31"); err != nil {
		t.Errorf("point-to-point placement rejected: %v", err)
	}
}

func TestNormalizeAddressRejectsAnythingButACanonicalPrivateHostAddress(t *testing.T) {
	address, err := NormalizeAddress("10.20.0.40")
	if err != nil || address != "10.20.0.40" {
		t.Fatalf("NormalizeAddress() = (%q, %v)", address, err)
	}
	for _, value := range []string{
		"", " 10.20.0.40", "10.20.0.40 ", "010.20.0.40", "10.20.0.40/32",
		"203.0.113.5", "::1", "10.20.0", "not-an-address",
	} {
		if _, err := NormalizeAddress(value); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("NormalizeAddress(%q) = %v, want rejection", value, err)
		}
	}
}

func observationAt(observedAt time.Time) Observation {
	observation := UnreportedObservation(testDecoyID, observedAt)
	observation.State = ObservedDeployed
	return observation
}

func TestValidateReportRejectsMalformedEdgeClaims(t *testing.T) {
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	valid := Report{
		ReportID: testReportID, ObservedAt: observedAt, DeviceID: testDeviceID,
		Observations: []Observation{observationAt(observedAt)},
	}
	if err := ValidateReport(valid); err != nil {
		t.Fatalf("ValidateReport() = %v", err)
	}

	cases := map[string]func(*Report){
		"report identity":     func(r *Report) { r.ReportID = "not-a-uuid" },
		"device identity":     func(r *Report) { r.DeviceID = "" },
		"missing observation": func(r *Report) { r.Observations[0].Conditions = nil },
		"condition order": func(r *Report) {
			r.Observations[0].Conditions[0].Type = ConditionPortResponding
		},
		"condition status": func(r *Report) { r.Observations[0].Conditions[2].Status = "Maybe" },
		"reason shape":     func(r *Report) { r.Observations[0].Conditions[1].Reason = "Not Observed" },
		"future interaction": func(r *Report) {
			future := observedAt.Add(time.Hour)
			r.Observations[0].LastInteractionAt = &future
		},
		"future transition": func(r *Report) {
			r.Observations[0].Conditions[3].LastTransitionTime = observedAt.Add(time.Hour)
		},
		"duplicate decoy": func(r *Report) {
			r.Observations = append(r.Observations, observationAt(observedAt))
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			report := Report{
				ReportID: testReportID, ObservedAt: observedAt, DeviceID: testDeviceID,
				Observations: []Observation{observationAt(observedAt)},
			}
			mutate(&report)
			if err := ValidateReport(report); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ValidateReport() = %v, want rejection", err)
			}
		})
	}
}

// SEC-06 is a Control Plane projection from device state. A device that has
// lost management is exactly the one whose claim about that cannot be trusted,
// so an Edge is never allowed to assert it.
func TestAnEdgeMayNotClaimItsOwnDecoysAreUnmanaged(t *testing.T) {
	observedAt := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	observation := observationAt(observedAt)
	observation.State = ObservedUnmanaged
	report := Report{
		ReportID: testReportID, ObservedAt: observedAt, DeviceID: testDeviceID,
		Observations: []Observation{observation},
	}
	if err := ValidateReport(report); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("an Edge claimed unmanaged and was accepted: %v", err)
	}
}

func TestResolvePackRejectsUnknownPacksAndFamilyDisagreement(t *testing.T) {
	entry, err := ResolvePack("smb-fileshare", "0.1.0", FamilySMB)
	if err != nil {
		t.Fatalf("ResolvePack() = %v", err)
	}
	if entry.InteractionLevel != InteractionLow {
		t.Fatalf("resolved interaction level = %q", entry.InteractionLevel)
	}
	if entry.Digest != "" {
		t.Fatalf("pack digest = %q; no manifest exists to hash yet", entry.Digest)
	}
	if _, err := ResolvePack("smb-fileshare", "9.9.9", FamilySMB); !errors.Is(err, ErrUnknownPack) {
		t.Fatalf("unknown version error = %v", err)
	}
	if _, err := ResolvePack("does-not-exist", "0.1.0", FamilySMB); !errors.Is(err, ErrUnknownPack) {
		t.Fatalf("unknown pack error = %v", err)
	}
	// A decoy whose declared family disagrees with its pack would render as one
	// thing in the console and behave as another.
	if _, err := ResolvePack("smb-fileshare", "0.1.0", FamilySSH); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("family mismatch error = %v", err)
	}
}

// decoys/index.json is the canonical pack index; the Go table is the read
// model. They live apart because decoys/ is outside this Go module and cannot
// be embedded, so this test is what keeps them from drifting.
func TestPackIndexMatchesCanonicalFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "decoys", "index.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read canonical pack index: %v", err)
	}
	var file struct {
		Schema string `json:"schema"`
		Packs  []struct {
			Pack             string  `json:"pack"`
			Version          string  `json:"version"`
			Digest           *string `json:"digest"`
			Family           string  `json:"family"`
			InteractionLevel string  `json:"interaction_level"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(content, &file); err != nil {
		t.Fatalf("parse canonical pack index: %v", err)
	}
	if file.Schema != "guardian.decoy.pack_index.v1" {
		t.Fatalf("canonical pack index schema = %q", file.Schema)
	}
	packs := Packs()
	if len(packs) != len(file.Packs) {
		t.Fatalf("Go index has %d packs, decoys/index.json has %d", len(packs), len(file.Packs))
	}
	for index, entry := range file.Packs {
		digest := ""
		if entry.Digest != nil {
			digest = *entry.Digest
		}
		want := PackEntry{
			Pack: entry.Pack, Version: entry.Version, Digest: digest,
			Family: Family(entry.Family), InteractionLevel: InteractionLevel(entry.InteractionLevel),
		}
		if packs[index] != want {
			t.Fatalf("pack %d: Go index %+v, canonical file %+v", index, packs[index], want)
		}
		if err := InteractionLevelFor(want.Family, want.InteractionLevel); err != nil {
			t.Fatalf("pack %s@%s violates INT-01: %v", want.Pack, want.Version, err)
		}
	}
	// One pack per DC-01 family, which is what the Phase 2 exit gate needs.
	families := make(map[Family]struct{}, len(packs))
	for _, entry := range packs {
		families[entry.Family] = struct{}{}
	}
	for _, family := range []Family{FamilySSH, FamilyHTTP, FamilyPostgres, FamilySMB} {
		if _, ok := families[family]; !ok {
			t.Errorf("no pack indexed for approved family %q", family)
		}
	}
}
