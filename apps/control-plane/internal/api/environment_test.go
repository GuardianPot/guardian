package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
)

const (
	testEnvironmentID = "0198dc8c-c600-7000-8000-000000000003"
	testZoneID        = "0198dc8c-c600-7000-8000-000000000004"
)

type environmentServiceStub struct {
	organization      func(context.Context) (environment.Organization, error)
	listEnvironments  func(context.Context, int32) ([]environment.Environment, error)
	getEnvironment    func(context.Context, string) (environment.Environment, error)
	createEnvironment func(context.Context, string, environment.Mutation) (environment.Environment, error)
	updateEnvironment func(context.Context, string, string, int64, environment.Mutation) (environment.Environment, error)
	listZones         func(context.Context, string, int32) ([]environment.Zone, error)
	getZone           func(context.Context, string, string) (environment.Zone, error)
	createZone        func(context.Context, string, string, string, environment.Mutation) (environment.Zone, error)
	updateZone        func(context.Context, string, string, string, string, int64, environment.Mutation) (environment.Zone, error)
	removeZone        func(context.Context, string, string, int64, environment.Mutation) error
}

func (s environmentServiceStub) Organization(ctx context.Context) (environment.Organization, error) {
	return s.organization(ctx)
}
func (s environmentServiceStub) ListEnvironments(ctx context.Context, limit int32) ([]environment.Environment, error) {
	return s.listEnvironments(ctx, limit)
}
func (s environmentServiceStub) Environment(ctx context.Context, id string) (environment.Environment, error) {
	return s.getEnvironment(ctx, id)
}
func (s environmentServiceStub) CreateEnvironment(ctx context.Context, name string, mutation environment.Mutation) (environment.Environment, error) {
	return s.createEnvironment(ctx, name, mutation)
}
func (s environmentServiceStub) UpdateEnvironment(ctx context.Context, id, name string, revision int64, mutation environment.Mutation) (environment.Environment, error) {
	return s.updateEnvironment(ctx, id, name, revision, mutation)
}
func (s environmentServiceStub) ListZones(ctx context.Context, environmentID string, limit int32) ([]environment.Zone, error) {
	return s.listZones(ctx, environmentID, limit)
}
func (s environmentServiceStub) Zone(ctx context.Context, environmentID, zoneID string) (environment.Zone, error) {
	return s.getZone(ctx, environmentID, zoneID)
}
func (s environmentServiceStub) CreateZone(ctx context.Context, environmentID, name, cidr string, mutation environment.Mutation) (environment.Zone, error) {
	return s.createZone(ctx, environmentID, name, cidr, mutation)
}
func (s environmentServiceStub) UpdateZone(ctx context.Context, environmentID, zoneID, name, cidr string, revision int64, mutation environment.Mutation) (environment.Zone, error) {
	return s.updateZone(ctx, environmentID, zoneID, name, cidr, revision, mutation)
}
func (s environmentServiceStub) RemoveZone(ctx context.Context, environmentID, zoneID string, revision int64, mutation environment.Mutation) error {
	return s.removeZone(ctx, environmentID, zoneID, revision, mutation)
}

func TestEnvironmentAPIIsTLSAndAuthenticationFailClosed(t *testing.T) {
	var called atomic.Bool
	service := environmentServiceStub{listEnvironments: func(context.Context, int32) ([]environment.Environment, error) {
		called.Store(true)
		return nil, nil
	}}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithEnvironmentService(service))

	plain := httptest.NewRequest(http.MethodGet, "http://guardian.test/v1/environments", nil)
	plainResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(plainResponse, plain)
	if plainResponse.Code != http.StatusUpgradeRequired {
		t.Fatalf("plain response = %d, want 426", plainResponse.Code)
	}

	secure := httptest.NewRequest(http.MethodGet, "https://guardian.test/v1/environments", nil)
	secure.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: strings.Repeat("s", 43)})
	secureResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(secureResponse, secure)
	if secureResponse.Code != http.StatusUnauthorized || called.Load() {
		t.Fatalf("default-deny response = %d, service called = %t", secureResponse.Code, called.Load())
	}
}

func TestEnvironmentAPIReadAndMutationContracts(t *testing.T) {
	service := environmentServiceStub{
		organization: func(context.Context) (environment.Organization, error) {
			return environment.Organization{OrganizationID: "org-1"}, nil
		},
		listEnvironments: func(_ context.Context, limit int32) ([]environment.Environment, error) {
			if limit != 75 {
				t.Fatalf("environment limit = %d", limit)
			}
			return []environment.Environment{{EnvironmentID: testEnvironmentID, Revision: 4}}, nil
		},
		getEnvironment: func(_ context.Context, id string) (environment.Environment, error) {
			return environment.Environment{EnvironmentID: id, Revision: 4}, nil
		},
		createEnvironment: func(_ context.Context, name string, mutation environment.Mutation) (environment.Environment, error) {
			if name != "Production" || mutation.ActorID != "owner-1" || mutation.RequestID != "request-7" {
				t.Fatalf("create input = %q %+v", name, mutation)
			}
			return environment.Environment{EnvironmentID: testEnvironmentID, DisplayName: name, Revision: 1}, nil
		},
		updateEnvironment: func(_ context.Context, id, name string, revision int64, mutation environment.Mutation) (environment.Environment, error) {
			if id != testEnvironmentID || name != "Production 2" || revision != 4 || mutation.ActorID != "owner-1" {
				t.Fatalf("update input = %q %q %d %+v", id, name, revision, mutation)
			}
			return environment.Environment{EnvironmentID: id, DisplayName: name, Revision: 5}, nil
		},
		listZones: func(_ context.Context, id string, limit int32) ([]environment.Zone, error) {
			if id != testEnvironmentID || limit != environment.DefaultListLimit {
				t.Fatalf("list zones input = %q %d", id, limit)
			}
			return []environment.Zone{}, nil
		},
		getZone: func(_ context.Context, environmentID, zoneID string) (environment.Zone, error) {
			return environment.Zone{EnvironmentID: environmentID, ZoneID: zoneID, Revision: 2}, nil
		},
		createZone: func(_ context.Context, environmentID, name, cidr string, mutation environment.Mutation) (environment.Zone, error) {
			if environmentID != testEnvironmentID || name != "Servers" || cidr != "10.20.0.0/24" || mutation.ActorID != "owner-1" {
				t.Fatalf("create zone input = %q %q %q %+v", environmentID, name, cidr, mutation)
			}
			return environment.Zone{EnvironmentID: environmentID, ZoneID: testZoneID, DisplayName: name, CIDR: cidr, Revision: 1}, nil
		},
		updateZone: func(_ context.Context, environmentID, zoneID, name, cidr string, revision int64, _ environment.Mutation) (environment.Zone, error) {
			if environmentID != testEnvironmentID || zoneID != testZoneID || name != "Server zone" || cidr != "10.21.0.0/24" || revision != 2 {
				t.Fatalf("update zone input = %q %q %q %q %d", environmentID, zoneID, name, cidr, revision)
			}
			return environment.Zone{EnvironmentID: environmentID, ZoneID: zoneID, DisplayName: name, CIDR: cidr, Revision: 3}, nil
		},
		removeZone: func(_ context.Context, environmentID, zoneID string, revision int64, mutation environment.Mutation) error {
			if environmentID != testEnvironmentID || zoneID != testZoneID || revision != 3 || mutation.ActorID != "owner-1" {
				t.Fatalf("remove zone input = %q %q %d %+v", environmentID, zoneID, revision, mutation)
			}
			return nil
		},
	}
	authorizer := EnvironmentAuthorizerFunc(func(_ context.Context, session, csrf, origin string, mutation bool) (string, error) {
		if session != strings.Repeat("s", 43) {
			t.Fatalf("session = %q", session)
		}
		if mutation && (csrf != strings.Repeat("c", 43) || origin != "https://guardian.test") {
			t.Fatalf("mutation proof = %q %q", csrf, origin)
		}
		if !mutation && (csrf != "" || origin != "") {
			t.Fatal("read forwarded mutation proof")
		}
		return "owner-1", nil
	})
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithEnvironmentService(service), WithEnvironmentAuthorizer(authorizer))

	for _, test := range []struct {
		method, path, body, ifMatch string
		status                      int
		etag                        string
	}{
		{http.MethodGet, "/v1/organization", "", "", 200, ""},
		{http.MethodGet, "/v1/environments?limit=75", "", "", 200, ""},
		{http.MethodGet, "/v1/environments/" + testEnvironmentID, "", "", 200, `"4"`},
		{http.MethodPost, "/v1/environments", `{"display_name":"Production"}`, "", 201, `"1"`},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID, `{"display_name":"Production 2"}`, `"4"`, 200, `"5"`},
		{http.MethodGet, "/v1/environments/" + testEnvironmentID + "/zones", "", "", 200, ""},
		{http.MethodGet, "/v1/environments/" + testEnvironmentID + "/zones/" + testZoneID, "", "", 200, `"2"`},
		{http.MethodPost, "/v1/environments/" + testEnvironmentID + "/zones", `{"display_name":"Servers","cidr":"10.20.0.0/24"}`, "", 201, `"1"`},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID + "/zones/" + testZoneID, `{"display_name":"Server zone","cidr":"10.21.0.0/24"}`, `"2"`, 200, `"3"`},
		{http.MethodDelete, "/v1/environments/" + testEnvironmentID + "/zones/" + testZoneID, "", `"3"`, 204, ""},
	} {
		request := authenticatedEnvironmentRequest(t, test.method, test.path, test.body, test.method != http.MethodGet)
		if test.ifMatch != "" {
			request.Header.Set("If-Match", test.ifMatch)
		}
		if test.method == http.MethodPost && test.path == "/v1/environments" {
			request.Header.Set("X-Request-ID", "request-7")
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("ETag") != test.etag || response.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s %s = %d ETag %q Cache-Control %q body %q", test.method, test.path, response.Code, response.Header().Get("ETag"), response.Header().Get("Cache-Control"), response.Body.String())
		}
	}
}

func TestEnvironmentAPIBoundsPreconditionsAndSanitizedErrors(t *testing.T) {
	secret := "postgres://guardian:secret@database/guardian"
	service := environmentServiceStub{
		listEnvironments: func(context.Context, int32) ([]environment.Environment, error) {
			return nil, errors.New(secret)
		},
		updateEnvironment: func(context.Context, string, string, int64, environment.Mutation) (environment.Environment, error) {
			return environment.Environment{}, environment.ErrPreconditionFailed
		},
		createEnvironment: func(context.Context, string, environment.Mutation) (environment.Environment, error) {
			return environment.Environment{}, nil
		},
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithEnvironmentService(service),
		WithEnvironmentAuthorizer(EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))

	for _, test := range []struct {
		method, path, body, ifMatch, requestID string
		status                                 int
	}{
		{http.MethodGet, "/v1/environments?limit=201", "", "", "", 400},
		{http.MethodGet, "/v1/environments?unknown=1", "", "", "", 400},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID, `{}`, "", "", 428},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID, `{"display_name":"x"}`, `W/"1"`, "", 400},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID, `{"display_name":"x"}`, `"01"`, "", 400},
		{http.MethodPatch, "/v1/environments/" + testEnvironmentID, `{"display_name":"x"}`, `"1"`, "", 412},
		{http.MethodPost, "/v1/environments", `{"display_name":"x","unknown":true}`, "", "", 400},
		{http.MethodPost, "/v1/environments", `{"display_name":"` + strings.Repeat("x", maximumEnvironmentRequestBytes) + `"}`, "", "", 400},
		{http.MethodPost, "/v1/environments", `{"display_name":"x"}`, "", strings.Repeat("r", environment.MaxRequestIDBytes+1), 400},
		{http.MethodDelete, "/v1/environments/" + testEnvironmentID, "", "", "", 405},
		{http.MethodGet, "/v1/environments", "", "", "", 500},
	} {
		request := authenticatedEnvironmentRequest(t, test.method, test.path, test.body, test.method != http.MethodGet)
		if test.ifMatch != "" {
			request.Header.Set("If-Match", test.ifMatch)
		}
		if test.requestID != "" {
			request.Header.Set("X-Request-ID", test.requestID)
		}
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("%s %s = %d body %q, want %d", test.method, test.path, response.Code, response.Body.String(), test.status)
		}
		if strings.Contains(response.Body.String(), secret) {
			t.Errorf("%s %s leaked database detail", test.method, test.path)
		}
	}
}

func authenticatedEnvironmentRequest(t *testing.T, method, path, body string, mutation bool) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, "https://guardian.test"+path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: authSessionCookieName, Value: strings.Repeat("s", 43)})
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if mutation {
		request.Header.Set(csrfHeaderName, strings.Repeat("c", 43))
		request.Header.Set("Origin", "https://guardian.test")
	}
	return request
}
