package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebConsoleServesAssetsAndClientRoutesWithoutAPIFallback(t *testing.T) {
	directory := t.TempDir()
	assets := filepath.Join(directory, "assets")
	if err := os.Mkdir(assets, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<!doctype html><title>Guardian</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "app-abc.js"), []byte("export {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithWebConsoleDirectory(directory))
	for _, test := range []struct {
		path, cache, body string
		status            int
	}{
		{path: "/", cache: "no-store", body: "Guardian", status: http.StatusOK},
		{path: "/environments/one/devices/two", cache: "no-store", body: "Guardian", status: http.StatusOK},
		{path: "/assets/app-abc.js", cache: "public, max-age=31536000, immutable", body: "export {}", status: http.StatusOK},
		{path: "/assets/missing.js", cache: "no-store", body: "not_found", status: http.StatusNotFound},
		{path: "/v1/does-not-exist", cache: "no-store", body: "not_found", status: http.StatusNotFound},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		if response.Code != test.status || response.Header().Get("Cache-Control") != test.cache || !strings.Contains(response.Body.String(), test.body) {
			t.Errorf("GET %s = %d cache=%q body=%q", test.path, response.Code, response.Header().Get("Cache-Control"), response.Body.String())
		}
		csp := response.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
			t.Errorf("GET %s CSP = %q", test.path, csp)
		}
	}
}

func TestWebConsoleCannotFollowSymlinkOutsideConfiguredRoot(t *testing.T) {
	directory := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("Guardian"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("must-not-leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(directory, "leak.txt")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}

	server := NewServer("127.0.0.1:0", nil, discardLogger(), WithWebConsoleDirectory(directory))
	request := httptest.NewRequest(http.MethodGet, "/leak.txt", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "must-not-leak") {
		t.Fatalf("symlink escape response = %d %q", response.Code, response.Body.String())
	}
}
