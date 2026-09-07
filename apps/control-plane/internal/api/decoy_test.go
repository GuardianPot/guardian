package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
)

const testDecoyID = "0198dc8c-c600-7000-8000-000000000005"

type decoyServiceStub struct {
	listDecoys   func(context.Context, string, int32) ([]deception.View, error)
	getDecoy     func(context.Context, string, string) (deception.View, error)
	createDecoy  func(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error)
	updateDecoy  func(context.Context, string, string, deception.Input, int64, deception.Mutation) (deception.Decoy, error)
	enableDecoy  func(context.Context, string, string, int64, deception.Mutation) (deception.Decoy, error)
	disableDecoy func(context.Context, string, string, int64, deception.Mutation) (deception.Decoy, error)
	removeDecoy  func(context.Context, string, string, int64, deception.Mutation) error
}

func (s decoyServiceStub) ListDecoys(ctx context.Context, id string, limit int32) ([]deception.View, error) {
	return s.listDecoys(ctx, id, limit)
}

func (s decoyServiceStub) Decoy(ctx context.Context, environmentID, decoyID string) (deception.View, error) {
	return s.getDecoy(ctx, environmentID, decoyID)
}

func (s decoyServiceStub) CreateDecoy(
	ctx context.Context, environmentID string, input deception.Input, mutation deception.Mutation,
) (deception.Decoy, error) {
	return s.createDecoy(ctx, environmentID, input, mutation)
}

func (s decoyServiceStub) UpdateDecoy(
	ctx context.Context, environmentID, decoyID string, input deception.Input,
	revision int64, mutation deception.Mutation,
) (deception.Decoy, error) {
	return s.updateDecoy(ctx, environmentID, decoyID, input, revision, mutation)
}

func (s decoyServiceStub) EnableDecoy(
	ctx context.Context, environmentID, decoyID string, revision int64, mutation deception.Mutation,
) (deception.Decoy, error) {
	return s.enableDecoy(ctx, environmentID, decoyID, revision, mutation)
}

func (s decoyServiceStub) DisableDecoy(
	ctx context.Context, environmentID, decoyID string, revision int64, mutation deception.Mutation,
) (deception.Decoy, error) {
	return s.disableDecoy(ctx, environmentID, decoyID, revision, mutation)
}

func (s decoyServiceStub) RemoveDecoy(
	ctx context.Context, environmentID, decoyID string, revision int64, mutation deception.Mutation,
) error {
	return s.removeDecoy(ctx, environmentID, decoyID, revision, mutation)
}

func sampleDecoy() deception.Decoy {
	return deception.Decoy{
		DecoyID: testDecoyID, EnvironmentID: testEnvironmentID, ZoneID: testZoneID,
		DisplayName: "Finance file server", Family: deception.FamilySMB,
		Persona: deception.PersonaWindowsFileServiceHost, InteractionLevel: deception.InteractionLow,
		Address: "10.20.0.40", Pack: "smb-fileshare", PackVersion: "0.1.0",
		DesiredState: deception.DesiredDeployed, Revision: 3,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
}

func decoyTestServer(service DecoyService) *Server {
	authorizer := EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
		return "owner-1", nil
	})
	return NewServer("127.0.0.1:0", nil, discardLogger(),
		WithDecoyService(service), WithEnvironmentAuthorizer(authorizer))
}

func TestDecoyAPIIsTLSAndAuthenticationFailClosed(t *testing.T) {
	var called atomic.Bool
	service := decoyServiceStub{listDecoys: func(context.Context, string, int32) ([]deception.View, error) {
		called.Store(true)
		return nil, nil
	}}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithDecoyService(service))

	plain := httptest.NewRequest(http.MethodGet, "http://guardian.test/v1/environments/"+testEnvironmentID+"/decoys", nil)
	plainResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(plainResponse, plain)
	if plainResponse.Code != http.StatusUpgradeRequired {
		t.Fatalf("plain response = %d, want 426", plainResponse.Code)
	}

	secure := httptest.NewRequest(http.MethodGet, "https://guardian.test/v1/environments/"+testEnvironmentID+"/decoys", nil)
	secure.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: strings.Repeat("s", 43)})
	secureResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(secureResponse, secure)
	if secureResponse.Code != http.StatusUnauthorized || called.Load() {
		t.Fatalf("default-deny response = %d, service called = %t", secureResponse.Code, called.Load())
	}
}

// The whole point of the contract: a decoy nothing has reported on reads as
// unknown, and the response never presents it as deployed or healthy.
func TestUnreportedDecoyReadsAsUnknownAndNeverAsDeployed(t *testing.T) {
	decoy := sampleDecoy()
	server := decoyTestServer(decoyServiceStub{
		getDecoy: func(context.Context, string, string) (deception.View, error) {
			return deception.View{
				Decoy:    decoy,
				Observed: deception.UnreportedObservation(decoy.DecoyID, decoy.CreatedAt),
			}, nil
		},
	})
	request := authenticatedEnvironmentRequest(t, http.MethodGet,
		"/v1/environments/"+testEnvironmentID+"/decoys/"+testDecoyID, "", false)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d", response.Code)
	}
	var body struct {
		Decoy struct {
			DesiredState string  `json:"desired_state"`
			PackDigest   *string `json:"pack_digest"`
			DisplayName  string  `json:"display_name"`
		} `json:"decoy"`
		Observed struct {
			State             string  `json:"observed_state"`
			LastInteractionAt *string `json:"last_interaction_at"`
			ReportedAt        *string `json:"reported_at"`
			Conditions        []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
		} `json:"observed"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Decoy.DesiredState != "deployed" {
		t.Fatalf("desired state = %q", body.Decoy.DesiredState)
	}
	if body.Observed.State != "unknown" {
		t.Fatalf("observed state = %q; nothing has reported on this decoy", body.Observed.State)
	}
	if body.Observed.LastInteractionAt != nil || body.Observed.ReportedAt != nil {
		t.Fatal("an unreported decoy carried report metadata")
	}
	if body.Decoy.PackDigest != nil {
		t.Fatalf("pack digest = %v; no manifest exists to hash yet", *body.Decoy.PackDigest)
	}
	if len(body.Observed.Conditions) != len(deception.ConditionTypes()) {
		t.Fatalf("conditions = %d, want the complete set", len(body.Observed.Conditions))
	}
	for index, condition := range body.Observed.Conditions {
		if condition.Type != string(deception.ConditionTypes()[index]) {
			t.Fatalf("condition %d = %q", index, condition.Type)
		}
		if condition.Status != "Unknown" {
			t.Fatalf("condition %q = %q, want Unknown", condition.Type, condition.Status)
		}
	}
	// Desired and observed are two sibling objects. No key at any level of the
	// response merges them into one "state".
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["state"]; ok {
		t.Fatal("response carries a merged state field")
	}
	if _, ok := raw["decoy"]; !ok {
		t.Fatal("response has no desired object")
	}
	if _, ok := raw["observed"]; !ok {
		t.Fatal("response has no observed object")
	}
}

// SEC-06: a revoked device's decoys are listed, and listed as unmanaged. Not
// healthy, not removed, and not silently missing.
func TestRevokedDeviceDecoysAreListedAsUnmanaged(t *testing.T) {
	reportedAt := time.Unix(1_700_000_000, 0).UTC()
	observation := deception.UnreportedObservation(testDecoyID, reportedAt)
	observation.State = deception.ObservedUnmanaged
	observation.ReportingDeviceID = "0198dc8c-c600-7000-8000-000000000009"
	observation.ReportedAt = &reportedAt
	server := decoyTestServer(decoyServiceStub{
		listDecoys: func(context.Context, string, int32) ([]deception.View, error) {
			return []deception.View{{Decoy: sampleDecoy(), Observed: observation}}, nil
		},
	})
	request := authenticatedEnvironmentRequest(t, http.MethodGet,
		"/v1/environments/"+testEnvironmentID+"/decoys", "", false)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d", response.Code)
	}
	var body struct {
		Decoys []struct {
			Decoy struct {
				DecoyID string `json:"decoy_id"`
			} `json:"decoy"`
			Observed struct {
				State string `json:"observed_state"`
			} `json:"observed"`
		} `json:"decoys"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Decoys) != 1 {
		t.Fatalf("listed %d decoys; a decoy Guardian cannot manage must still be visible", len(body.Decoys))
	}
	if body.Decoys[0].Observed.State != "unmanaged" {
		t.Fatalf("observed state = %q, want unmanaged", body.Decoys[0].Observed.State)
	}
}

func TestDecoyWriteContractsAndOptimisticConcurrency(t *testing.T) {
	var (
		createdInput deception.Input
		enabled      int
		disabled     int
		removed      int
	)
	service := decoyServiceStub{
		getDecoy: func(context.Context, string, string) (deception.View, error) {
			return deception.View{
				Decoy:    sampleDecoy(),
				Observed: deception.UnreportedObservation(testDecoyID, time.Unix(0, 0).UTC()),
			}, nil
		},
		createDecoy: func(_ context.Context, environmentID string, input deception.Input, mutation deception.Mutation) (deception.Decoy, error) {
			if environmentID != testEnvironmentID || mutation.ActorID != "owner-1" {
				t.Fatalf("create scope = %q %+v", environmentID, mutation)
			}
			createdInput = input
			decoy := sampleDecoy()
			decoy.Revision = 1
			return decoy, nil
		},
		updateDecoy: func(_ context.Context, _, _ string, _ deception.Input, revision int64, _ deception.Mutation) (deception.Decoy, error) {
			if revision != 3 {
				t.Fatalf("update revision = %d", revision)
			}
			decoy := sampleDecoy()
			decoy.Revision = 4
			return decoy, nil
		},
		enableDecoy: func(_ context.Context, _, _ string, revision int64, _ deception.Mutation) (deception.Decoy, error) {
			enabled++
			if revision != 3 {
				t.Fatalf("enable revision = %d", revision)
			}
			return sampleDecoy(), nil
		},
		disableDecoy: func(_ context.Context, _, _ string, revision int64, _ deception.Mutation) (deception.Decoy, error) {
			disabled++
			if revision != 3 {
				t.Fatalf("disable revision = %d", revision)
			}
			decoy := sampleDecoy()
			decoy.DesiredState = deception.DesiredDisabled
			return decoy, nil
		},
		removeDecoy: func(_ context.Context, _, _ string, revision int64, mutation deception.Mutation) error {
			removed++
			if revision != 3 || mutation.ActorID != "owner-1" {
				t.Fatalf("remove input = %d %+v", revision, mutation)
			}
			return nil
		},
	}
	server := decoyTestServer(service)
	base := "/v1/environments/" + testEnvironmentID + "/decoys"
	body := `{"zone_id":"` + testZoneID + `","display_name":"Finance file server",` +
		`"family":"smb","persona":"windows_file_service_host","address":"10.20.0.40",` +
		`"pack":"smb-fileshare","pack_version":"0.1.0"}`

	for _, test := range []struct {
		method, path, body, ifMatch string
		status                      int
		etag                        string
	}{
		{http.MethodPost, base, body, "", 201, `"1"`},
		{http.MethodPatch, base + "/" + testDecoyID, body, `"3"`, 200, `"4"`},
		{http.MethodPost, base + "/" + testDecoyID + "/enable", "", `"3"`, 200, `"3"`},
		{http.MethodPost, base + "/" + testDecoyID + "/disable", "", `"3"`, 200, `"3"`},
		{http.MethodDelete, base + "/" + testDecoyID, "", `"3"`, 204, ""},
	} {
		request := authenticatedEnvironmentRequest(t, test.method, test.path, test.body, true)
		if test.ifMatch != "" {
			request.Header.Set("If-Match", test.ifMatch)
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("%s %s = %d, want %d (%s)", test.method, test.path, response.Code, test.status, response.Body)
		}
		if got := response.Header().Get("ETag"); got != test.etag {
			t.Fatalf("%s %s ETag = %q, want %q", test.method, test.path, got, test.etag)
		}
	}
	if createdInput.Pack != "smb-fileshare" || createdInput.Address != "10.20.0.40" {
		t.Fatalf("create input = %+v", createdInput)
	}
	if enabled != 1 || disabled != 1 || removed != 1 {
		t.Fatalf("transitions = enable %d disable %d remove %d", enabled, disabled, removed)
	}

	// Every state-changing operation on an existing decoy requires the strong
	// revision ETag, the same as the zone domain.
	for _, test := range []struct{ method, path string }{
		{http.MethodPatch, base + "/" + testDecoyID},
		{http.MethodPost, base + "/" + testDecoyID + "/enable"},
		{http.MethodPost, base + "/" + testDecoyID + "/disable"},
		{http.MethodDelete, base + "/" + testDecoyID},
	} {
		request := authenticatedEnvironmentRequest(t, test.method, test.path, "", true)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusPreconditionRequired {
			t.Fatalf("%s %s without If-Match = %d, want 428", test.method, test.path, response.Code)
		}
	}
}

// There is no field in the write contract for a runtime detail, and an unknown
// field is rejected rather than ignored.
func TestDecoyWriteRejectsRuntimeFieldsAndUnknownKeys(t *testing.T) {
	var called atomic.Bool
	server := decoyTestServer(decoyServiceStub{
		createDecoy: func(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error) {
			called.Store(true)
			return sampleDecoy(), nil
		},
	})
	base := "/v1/environments/" + testEnvironmentID + "/decoys"
	for _, body := range []string{
		`{"zone_id":"` + testZoneID + `","display_name":"x","family":"smb",` +
			`"persona":"windows_file_service_host","address":"10.20.0.40",` +
			`"pack":"smb-fileshare","pack_version":"0.1.0","image":"docker.io/evil:latest"}`,
		`{"zone_id":"` + testZoneID + `","display_name":"x","family":"smb",` +
			`"persona":"windows_file_service_host","address":"10.20.0.40",` +
			`"pack":"smb-fileshare","pack_version":"0.1.0","command":["/bin/sh"]}`,
		`{"zone_id":"` + testZoneID + `","display_name":"x","family":"smb",` +
			`"persona":"windows_file_service_host","address":"10.20.0.40",` +
			`"pack":"smb-fileshare","pack_version":"0.1.0","pack_digest":"sha256:` + strings.Repeat("a", 64) + `"}`,
		`{"zone_id":"` + testZoneID + `","display_name":"x","family":"smb",` +
			`"persona":"windows_file_service_host","address":"10.20.0.40",` +
			`"pack":"smb-fileshare","pack_version":"0.1.0","interaction_level":"medium"}`,
	} {
		request := authenticatedEnvironmentRequest(t, http.MethodPost, base, body, true)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("request with a runtime field = %d, want 400: %s", response.Code, body)
		}
	}
	if called.Load() {
		t.Fatal("a request carrying a runtime field reached the service")
	}
}
