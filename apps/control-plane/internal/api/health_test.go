package api

import (
	"context"
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
