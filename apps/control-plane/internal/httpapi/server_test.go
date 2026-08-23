package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type readinessFunc func(context.Context) error

func (fn readinessFunc) Ready(ctx context.Context) error { return fn(ctx) }

func TestHealthEndpointsAreTruthfulAndSanitized(t *testing.T) {
	secret := "postgres://guardian:do-not-leak@db/guardian"
	server := New("127.0.0.1:0", readinessFunc(func(context.Context) error {
		return errors.New(secret)
	}), slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, test := range []struct {
		path       string
		statusCode int
		status     string
	}{
		{path: "/livez", statusCode: http.StatusOK, status: "live"},
		{path: "/readyz", statusCode: http.StatusServiceUnavailable, status: "not_ready"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.statusCode || !strings.Contains(response.Body.String(), test.status) {
			t.Fatalf("GET %s = %d %q", test.path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), secret) {
			t.Fatalf("GET %s leaked an internal database error", test.path)
		}
	}
}

func TestReadyEndpointReturnsReady(t *testing.T) {
	server := New("127.0.0.1:0", readinessFunc(func(context.Context) error { return nil }), slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ready") {
		t.Fatalf("GET /readyz = %d %q", response.Code, response.Body.String())
	}
}

func TestStartPreservesInitialBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	server := New(occupied.Addr().String(), readinessFunc(func(context.Context) error { return nil }), slog.Default())
	first := server.Start()
	second := server.Start()
	if first == nil || second == nil {
		t.Fatalf("Start() errors = (%v, %v), want persistent bind failure", first, second)
	}
}
