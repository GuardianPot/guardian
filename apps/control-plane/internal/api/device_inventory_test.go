package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
)

type deviceInventoryStub struct {
	list func(context.Context, string) ([]devices.InventoryDevice, error)
	get  func(context.Context, string, string) (devices.InventoryDevice, error)
}

func (stub deviceInventoryStub) ListDevices(ctx context.Context, environmentID string) ([]devices.InventoryDevice, error) {
	return stub.list(ctx, environmentID)
}

func (stub deviceInventoryStub) Device(ctx context.Context, environmentID, deviceID string) (devices.InventoryDevice, error) {
	return stub.get(ctx, environmentID, deviceID)
}

func TestDeviceInventoryAuthorizesBeforeReadingAndReturnsNoStore(t *testing.T) {
	var reads atomic.Int32
	service := deviceInventoryStub{
		list: func(context.Context, string) ([]devices.InventoryDevice, error) {
			reads.Add(1)
			return []devices.InventoryDevice{{
				DeviceID: "018f1f7e-6d31-7cc5-8db8-17547f78e6c2", EnvironmentID: "018f1f7e-6d31-7cc5-8db8-17547f78e6c1",
				DisplayName: "edge <script>alert(1)</script>", State: devices.DeviceActive, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(2, 0).UTC(),
			}}, nil
		},
		get: func(context.Context, string, string) (devices.InventoryDevice, error) {
			reads.Add(1)
			return devices.InventoryDevice{}, devices.ErrNotFound
		},
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithDeviceInventoryService(service))
	request := httptest.NewRequest(http.MethodGet, "/v1/environments/018f1f7e-6d31-7cc5-8db8-17547f78e6c1/devices", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || reads.Load() != 0 {
		t.Fatalf("unauthorized response = %d, reads = %d", response.Code, reads.Load())
	}

	server = NewServer("127.0.0.1:0", nil, discardLogger(), WithDeviceInventoryService(service),
		WithDeviceAdminAuthorizer(DeviceAdminAuthorizerFunc(func(context.Context, string, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))
	request = httptest.NewRequest(http.MethodGet, "/v1/environments/018f1f7e-6d31-7cc5-8db8-17547f78e6c1/devices", nil)
	request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: strings.Repeat("a", 43)})
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || reads.Load() != 1 || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("authorized response = %d headers=%v reads=%d body=%s", response.Code, response.Header(), reads.Load(), response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `edge \u003cscript\u003ealert(1)\u003c/script\u003e`) || strings.Contains(response.Body.String(), "<script>") {
		t.Fatalf("JSON inventory text was not safely encoded: %s", response.Body.String())
	}
}

func TestDeviceInventoryMapsNotFoundWithoutLeakingErrors(t *testing.T) {
	secret := "postgres://do-not-leak"
	service := deviceInventoryStub{
		list: func(context.Context, string) ([]devices.InventoryDevice, error) { return nil, errors.New(secret) },
		get: func(context.Context, string, string) (devices.InventoryDevice, error) {
			return devices.InventoryDevice{}, devices.ErrNotFound
		},
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithDeviceInventoryService(service),
		WithDeviceAdminAuthorizer(DeviceAdminAuthorizerFunc(func(context.Context, string, string, string, string, bool) (string, error) {
			return "owner-1", nil
		})))
	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/v1/environments/018f1f7e-6d31-7cc5-8db8-17547f78e6c1/devices", want: http.StatusInternalServerError},
		{path: "/v1/environments/018f1f7e-6d31-7cc5-8db8-17547f78e6c1/devices/018f1f7e-6d31-7cc5-8db8-17547f78e6c2", want: http.StatusNotFound},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: strings.Repeat("a", 43)})
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.want || strings.Contains(response.Body.String(), secret) {
			t.Fatalf("GET %s = %d %q", test.path, response.Code, response.Body.String())
		}
	}
}
