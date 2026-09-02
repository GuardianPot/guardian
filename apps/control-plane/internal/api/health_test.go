package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/health"
)

const healthAPIDeviceID = "0198dc8c-c600-7000-8000-000000000004"

type healthServiceStub struct {
	device      func(context.Context, string) (health.View, error)
	environment func(context.Context, string) (health.View, error)
}

func (stub healthServiceStub) Device(ctx context.Context, id string) (health.View, error) {
	return stub.device(ctx, id)
}
func (stub healthServiceStub) Environment(ctx context.Context, id string) (health.View, error) {
	return stub.environment(ctx, id)
}

func TestHealthReadsRequireOwnerSessionAndReturnNoStoreTruth(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	view := health.View{Aggregate: health.ViewAggregate{Status: health.StatusUnknown, Reason: "no_active_devices"}, ReceivedAt: now}
	service := healthServiceStub{
		device:      func(context.Context, string) (health.View, error) { return view, nil },
		environment: func(context.Context, string) (health.View, error) { return view, nil },
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithHealthService(service),
		WithEnvironmentAuthorizer(EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))

	unauthorized := httptest.NewRequest(http.MethodGet, "https://guardian.test/v1/devices/"+healthAPIDeviceID+"/health", nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorizedResponse.Code)
	}

	for _, path := range []string{
		"/v1/devices/" + healthAPIDeviceID + "/health",
		"/v1/environments/" + testEnvironmentID + "/health",
	} {
		request := authenticatedEnvironmentRequest(t, http.MethodGet, path, "", false)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" || !strings.Contains(response.Body.String(), "no_active_devices") {
			t.Fatalf("GET %s = %d headers=%v body=%q", path, response.Code, response.Header(), response.Body.String())
		}
	}
}

// HostileHealthMessage is bounded, secret-free markup that the canonical
// contract accepts. It must survive the API as inert JSON text.
const hostileHealthMessage = `<img src=x onerror="alert(1)"><script>alert(2)</script>"><svg onload=alert(3)>`

func TestHealthReadEncodesHostileMessagesAsInertJSONText(t *testing.T) {
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	condition := health.Condition{
		Type:               health.TypeSpoolHealthy,
		Status:             health.StatusFalse,
		Reason:             "capacity_critical",
		Message:            hostileHealthMessage,
		LastTransitionTime: now,
	}
	if err := condition.Validate(); err != nil {
		t.Fatalf("canonical contract rejected bounded hostile message: %v", err)
	}
	view := health.View{
		Aggregate:  health.ViewAggregate{Status: health.StatusFalse, BlockingType: health.TypeSpoolHealthy, Reason: "capacity_critical"},
		Conditions: []health.SourcedCondition{{Condition: condition, SourceDeviceID: healthAPIDeviceID}},
		ReceivedAt: now,
	}
	service := healthServiceStub{
		device:      func(context.Context, string) (health.View, error) { return view, nil },
		environment: func(context.Context, string) (health.View, error) { return view, nil },
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithHealthService(service),
		WithEnvironmentAuthorizer(EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))

	for _, path := range []string{
		"/v1/devices/" + healthAPIDeviceID + "/health",
		"/v1/environments/" + testEnvironmentID + "/health",
	} {
		request := authenticatedEnvironmentRequest(t, http.MethodGet, path, "", false)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("GET %s content type = %q", path, got)
		}
		body := response.Body.String()
		// No raw delimiter survives encoding: json.Marshal escapes them, and a
		// custom encoder with SetEscapeHTML(false) would emit the tags intact.
		for _, raw := range []string{"<img", "<script", "<svg", "\">"} {
			if strings.Contains(body, raw) {
				t.Fatalf("GET %s emitted unescaped %q in %q", path, raw, body)
			}
		}
		// Escaping is lossless rather than stripping or entity rewriting: the
		// decoded message must equal the original byte-for-byte.
		var decoded health.View
		if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("GET %s response is not valid JSON: %v", path, err)
		}
		if len(decoded.Conditions) != 1 || decoded.Conditions[0].Message != hostileHealthMessage {
			t.Fatalf("GET %s lost or altered the message: %+v", path, decoded.Conditions)
		}
	}
}

func TestHealthReadSanitizesRepositoryFailure(t *testing.T) {
	secret := "postgres://guardian:do-not-leak@database/guardian"
	service := healthServiceStub{
		device:      func(context.Context, string) (health.View, error) { return health.View{}, errors.New(secret) },
		environment: func(context.Context, string) (health.View, error) { return health.View{}, health.ErrNotFound },
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithHealthService(service),
		WithEnvironmentAuthorizer(EnvironmentAuthorizerFunc(func(context.Context, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))
	for _, test := range []struct {
		path string
		want int
	}{
		{"/v1/devices/" + healthAPIDeviceID + "/health", http.StatusInternalServerError},
		{"/v1/environments/" + testEnvironmentID + "/health", http.StatusNotFound},
	} {
		request := authenticatedEnvironmentRequest(t, http.MethodGet, test.path, "", false)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.want || strings.Contains(response.Body.String(), secret) {
			t.Fatalf("GET %s = %d body=%q", test.path, response.Code, response.Body.String())
		}
	}
}
