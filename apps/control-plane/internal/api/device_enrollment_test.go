package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
)

type deviceServiceStub struct {
	create   func(context.Context, string, string, string) (devices.EnrollmentToken, error)
	reenroll func(context.Context, string, string, string) (devices.EnrollmentToken, error)
	enroll   func(context.Context, string, string, []byte) (devices.EnrollmentResult, error)
}

func (s deviceServiceStub) CreateReenrollmentToken(ctx context.Context, environmentID, deviceID, actor string) (devices.EnrollmentToken, error) {
	if s.reenroll == nil {
		return devices.EnrollmentToken{}, nil
	}
	return s.reenroll(ctx, environmentID, deviceID, actor)
}

func (s deviceServiceStub) CreateEnrollmentToken(ctx context.Context, environmentID, name, actor string) (devices.EnrollmentToken, error) {
	return s.create(ctx, environmentID, name, actor)
}
func (deviceServiceStub) ListEnrollmentTokens(context.Context, string) ([]devices.TokenSummary, error) {
	return nil, nil
}
func (deviceServiceStub) RevokeEnrollmentToken(context.Context, string, string, string) error {
	return nil
}
func (s deviceServiceStub) Enroll(ctx context.Context, source, token string, csr []byte) (devices.EnrollmentResult, error) {
	return s.enroll(ctx, source, token, csr)
}
func (deviceServiceStub) Rotate(context.Context, string, string, []byte) (devices.EnrollmentResult, error) {
	return devices.EnrollmentResult{}, nil
}
func (deviceServiceStub) SetDeviceState(context.Context, string, string, devices.DeviceState, string) error {
	return nil
}
func (deviceServiceStub) VerifyCertificate(context.Context, *x509.Certificate) (string, string, error) {
	return "", "", nil
}

func TestDeviceAdminMutationDefaultsToDenyBeforeBodyOrService(t *testing.T) {
	var calls atomic.Int32
	service := deviceServiceStub{create: func(context.Context, string, string, string) (devices.EnrollmentToken, error) {
		calls.Add(1)
		return devices.EnrollmentToken{}, nil
	}}
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(), WithDeviceService(service))
	request := httptest.NewRequest(http.MethodPost,
		"/v1/environments/0198dc8c-c600-7000-8000-000000000003/enrollment-tokens",
		strings.NewReader(`{"device_name":"edge-one"}`))
	request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: "opaque-session"})
	request.Header.Set(csrfHeaderName, "csrf")
	request.Header.Set("Origin", "https://console.guardian.test")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || calls.Load() != 0 {
		t.Fatalf("response = %d %q, calls = %d", response.Code, response.Body.String(), calls.Load())
	}
}

func TestAuthorizedDeviceTokenCreationReturnsSecretOnceWithNoStore(t *testing.T) {
	expires := time.Date(2026, 8, 24, 20, 15, 0, 0, time.UTC)
	service := deviceServiceStub{create: func(_ context.Context, environmentID, name, actor string) (devices.EnrollmentToken, error) {
		if name != "edge-one" || actor != "owner-1" {
			t.Fatalf("create input = %q %q %q", environmentID, name, actor)
		}
		return devices.EnrollmentToken{
			TokenID: "0198dc8c-c600-7000-8000-000000000005", DeviceID: "0198dc8c-c600-7000-8000-000000000006",
			EnvironmentID: environmentID, DeviceName: name, Token: "one-time-secret", ExpiresAt: expires,
		}, nil
	}}
	authorizer := DeviceAdminAuthorizerFunc(func(_ context.Context, session, environment, csrf, origin string, mutation bool) (string, error) {
		if session != "session" || csrf != "csrf" || origin != "https://console.guardian.test" || !mutation {
			t.Fatalf("authorization input = %q %q %q %q %t", session, environment, csrf, origin, mutation)
		}
		return "owner-1", nil
	})
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(),
		WithDeviceService(service), WithDeviceAdminAuthorizer(authorizer))
	request := httptest.NewRequest(http.MethodPost,
		"/v1/environments/0198dc8c-c600-7000-8000-000000000003/enrollment-tokens",
		strings.NewReader(`{"device_name":"edge-one"}`))
	request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: "session"})
	request.Header.Set(csrfHeaderName, "csrf")
	request.Header.Set("Origin", "https://console.guardian.test")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(response.Body.String(), "one-time-secret") {
		t.Fatalf("response = %d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestAuthorizedReenrollmentKeepsStableDeviceID(t *testing.T) {
	const environmentID = "0198dc8c-c600-7000-8000-000000000003"
	const deviceID = "0198dc8c-c600-7000-8000-000000000004"
	service := deviceServiceStub{reenroll: func(_ context.Context, environment, device, actor string) (devices.EnrollmentToken, error) {
		if environment != environmentID || device != deviceID || actor != "owner-1" {
			t.Fatalf("re-enrollment input = %q %q %q", environment, device, actor)
		}
		return devices.EnrollmentToken{DeviceID: device, EnvironmentID: environment, Token: "new-one-time-secret"}, nil
	}}
	authorizer := DeviceAdminAuthorizerFunc(func(context.Context, string, string, string, string, bool) (string, error) {
		return "owner-1", nil
	})
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(),
		WithDeviceService(service), WithDeviceAdminAuthorizer(authorizer))
	request := httptest.NewRequest(http.MethodPost,
		"/v1/environments/"+environmentID+"/devices/"+deviceID+"/re-enrollment-token", nil)
	request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: "session"})
	request.Header.Set(csrfHeaderName, "csrf")
	request.Header.Set("Origin", "https://console.guardian.test")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated || response.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(response.Body.String(), `"device_id":"`+deviceID+`"`) {
		t.Fatalf("response = %d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestEnrollmentRequiresTLSSingleBearerAndUsesGenericDenial(t *testing.T) {
	var calls atomic.Int32
	service := deviceServiceStub{enroll: func(context.Context, string, string, []byte) (devices.EnrollmentResult, error) {
		calls.Add(1)
		return devices.EnrollmentResult{}, devices.ErrTokenConsumed
	}}
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(), WithDeviceService(service))
	newRequest := func() *http.Request {
		request := httptest.NewRequest(http.MethodPost, "/v1/enrollments", strings.NewReader(`{"csr_pem":"csr"}`))
		request.Header.Set("Authorization", "Bearer AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
		return request
	}
	plain := httptest.NewRecorder()
	server.Handler().ServeHTTP(plain, newRequest())
	if plain.Code != http.StatusUpgradeRequired || calls.Load() != 0 {
		t.Fatalf("plain response = %d, calls=%d", plain.Code, calls.Load())
	}
	tlsRequest := newRequest()
	tlsRequest.TLS = &tls.ConnectionState{}
	tlsResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(tlsResponse, tlsRequest)
	if tlsResponse.Code != http.StatusUnauthorized || !strings.Contains(tlsResponse.Body.String(), "enrollment_denied") ||
		strings.Contains(tlsResponse.Body.String(), "consumed") || calls.Load() != 1 {
		t.Fatalf("TLS response = %d %q, calls=%d", tlsResponse.Code, tlsResponse.Body.String(), calls.Load())
	}
}

func TestEnrollmentRateLimitIsBoundedAndGeneric(t *testing.T) {
	service := deviceServiceStub{enroll: func(context.Context, string, string, []byte) (devices.EnrollmentResult, error) {
		return devices.EnrollmentResult{}, devices.ErrEnrollmentRateLimited
	}}
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(), WithDeviceService(service))
	request := httptest.NewRequest(http.MethodPost, "/v1/enrollments", strings.NewReader(`{"csr_pem":"csr"}`))
	request.TLS = &tls.ConnectionState{}
	request.Header.Set("Authorization", "Bearer AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" ||
		!strings.Contains(response.Body.String(), "enrollment_denied") {
		t.Fatalf("response = %d headers=%v body=%q", response.Code, response.Header(), response.Body.String())
	}
}

func TestRotationAcceptsVerifiedLeafWithPresentedChain(t *testing.T) {
	server := NewServer("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), discardLogger(),
		WithDeviceService(deviceServiceStub{}))
	request := httptest.NewRequest(http.MethodPost, "/v1/device-certificates:rotate", strings.NewReader(`{"csr_pem":"csr"}`))
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{}, {}}}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("rotation response = %d %q", response.Code, response.Body.String())
	}
}
