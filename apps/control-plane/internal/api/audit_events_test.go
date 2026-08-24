package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
)

type auditReaderFunc func(context.Context, audit.ListQuery) (audit.Page, error)

func (fn auditReaderFunc) List(ctx context.Context, query audit.ListQuery) (audit.Page, error) {
	return fn(ctx, query)
}

type readinessAuditReader struct {
	auditReaderFunc
}

func (readinessAuditReader) Ready(context.Context) error { return nil }

func TestAuditReadDefaultsToUnauthorizedBeforeReader(t *testing.T) {
	var reads atomic.Int32
	reader := auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
		reads.Add(1)
		return audit.Page{}, nil
	})
	server := newAuditTestServer(reader)
	request := newAuditRequest(http.MethodGet, "/v1/audit/events", "opaque-session")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), "unauthorized") {
		t.Fatalf("GET audit status = %d %q", response.Code, response.Body.String())
	}
	if reads.Load() != 0 {
		t.Fatalf("audit reader calls = %d, want zero", reads.Load())
	}
}

func TestAuditReadRequiresExactlyOneNonEmptySessionCookie(t *testing.T) {
	tests := []struct {
		name    string
		cookies []string
	}{
		{name: "missing"},
		{name: "empty", cookies: []string{""}},
		{name: "whitespace", cookies: []string{" "}},
		{name: "oversized", cookies: []string{strings.Repeat("s", maximumAuditSessionSize+1)}},
		{name: "duplicate", cookies: []string{"first", "second"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var authorizations atomic.Int32
			var reads atomic.Int32
			server := newAuditTestServer(
				auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
					reads.Add(1)
					return audit.Page{}, nil
				}),
				WithAuditAuthorizer(AuditAuthorizerFunc(func(context.Context, string) error {
					authorizations.Add(1)
					return nil
				})),
			)
			request := httptest.NewRequest(http.MethodGet, "/v1/audit/events", nil)
			for _, value := range test.cookies {
				request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: value})
			}
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", response.Code)
			}
			if authorizations.Load() != 0 || reads.Load() != 0 {
				t.Fatalf("calls = authorizer:%d reader:%d, want zero", authorizations.Load(), reads.Load())
			}
		})
	}
}

func TestAuditReadPassesOnlyOpaqueSessionToAuthorizer(t *testing.T) {
	session := strings.Repeat("s", maximumAuditSessionSize)
	var authorizedSession string
	server := newAuditTestServer(
		auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
			return audit.Page{Records: []audit.Record{}}, nil
		}),
		WithAuditAuthorizer(AuditAuthorizerFunc(func(_ context.Context, actual string) error {
			authorizedSession = actual
			return nil
		})),
	)
	request := newAuditRequest(http.MethodGet, "/v1/audit/events", session)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	if authorizedSession != session {
		t.Fatalf("authorized session = %q", authorizedSession)
	}
	if strings.Contains(response.Body.String(), session) {
		t.Fatal("response leaked the opaque session")
	}
}

func TestAuditReadUsesDefaultsAndOmitsInternalSequence(t *testing.T) {
	occurredAt := time.Date(2026, 8, 24, 12, 0, 0, 123000000, time.UTC)
	recordedAt := occurredAt.Add(time.Second)
	snapshot, err := audit.NewSnapshot(map[string]any{"result": "denied", "token": "must-not-leak"})
	if err != nil {
		t.Fatal(err)
	}
	next, err := audit.NewCursor(987654321, 987654321)
	if err != nil {
		t.Fatal(err)
	}
	nextCursor, err := next.Encode()
	if err != nil {
		t.Fatal(err)
	}
	reader := auditReaderFunc(func(_ context.Context, query audit.ListQuery) (audit.Page, error) {
		if query.Limit != defaultAuditPageLimit || query.Cursor != (audit.Cursor{}) {
			t.Fatalf("query defaults = %+v", query)
		}
		return audit.Page{
			Records: []audit.Record{{
				EventID:    "0198dc8c-c600-7000-8000-000000000001",
				Sequence:   987654321,
				RecordedAt: recordedAt,
				Event: audit.Event{
					OccurredAt:    occurredAt,
					Actor:         audit.Actor{Type: audit.ActorType("system"), ID: "control-plane"},
					Action:        audit.Action("security.action.denied"),
					Object:        audit.ObjectRef{Type: audit.ObjectType("security_action"), ID: "request-7"},
					CorrelationID: "correlation-7",
					RequestID:     "request-7",
					Before:        &snapshot,
				},
			}},
			NextCursor: nextCursor,
		}, nil
	})
	server := newAuditTestServer(reader, allowAuditReads())
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{
		`"event_id":"0198dc8c-c600-7000-8000-000000000001"`,
		`"occurred_at":"2026-08-24T12:00:00.123Z"`,
		`"recorded_at":"2026-08-24T12:00:01.123Z"`,
		`"type":"system"`,
		`"action":"security.action.denied"`,
		`"correlation_id":"correlation-7"`,
		`"before":{"schema":"guardian.audit.snapshot.v1","data":{"result":"denied","token":"[REDACTED]"}}`,
		`"next_cursor":"` + nextCursor + `"`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("response %q does not contain %q", body, expected)
		}
	}
	if strings.Contains(body, "sequence") || strings.Contains(body, "987654321") {
		t.Fatalf("response exposed internal sequence: %q", body)
	}
	if strings.Contains(body, "must-not-leak") {
		t.Fatalf("response exposed a redacted snapshot value: %q", body)
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("response headers = %#v", response.Header())
	}
}

func TestAuditReadParsesVersionedCursor(t *testing.T) {
	want, err := audit.NewCursor(900, 700)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := want.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var received audit.Cursor
	server := newAuditTestServer(
		auditReaderFunc(func(_ context.Context, query audit.ListQuery) (audit.Page, error) {
			received = query.Cursor
			return audit.Page{}, nil
		}),
		allowAuditReads(),
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events?cursor="+encoded, "session"))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	if received != want {
		t.Fatalf("cursor = %+v, want %+v", received, want)
	}
}

func TestNewServerDiscoversAuditReaderFromReadiness(t *testing.T) {
	var reads atomic.Int32
	dependency := readinessAuditReader{auditReaderFunc: func(context.Context, audit.ListQuery) (audit.Page, error) {
		reads.Add(1)
		return audit.Page{}, nil
	}}
	server := NewServer("127.0.0.1:0", dependency, discardLogger(), allowAuditReads())
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusOK || reads.Load() != 1 {
		t.Fatalf("status = %d reader calls = %d body = %q", response.Code, reads.Load(), response.Body.String())
	}
}

func TestAuditReadAppliesExactFiltersAndMaximumLimit(t *testing.T) {
	var received audit.ListQuery
	server := newAuditTestServer(
		auditReaderFunc(func(_ context.Context, query audit.ListQuery) (audit.Page, error) {
			received = query
			return audit.Page{}, nil
		}),
		allowAuditReads(),
	)
	request := newAuditRequest(
		http.MethodGet,
		"/v1/audit/events?limit=200&action=auth.login.failed&correlation_id=correlation-1&object_type=user&object_id=user-1",
		"session",
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	if received.Limit != maximumAuditPageLimit || received.Action != audit.Action("auth.login.failed") ||
		received.CorrelationID != "correlation-1" || received.ObjectType != audit.ObjectType("user") ||
		received.ObjectID != "user-1" {
		t.Fatalf("reader query = %+v", received)
	}
}

func TestAuditReadRejectsInvalidOrAmbiguousQueryBeforeReader(t *testing.T) {
	queries := []string{
		"?unknown=value",
		"?limit=",
		"?limit=0",
		"?limit=-1",
		"?limit=201",
		"?limit=not-a-number",
		"?limit=1&limit=2",
		"?limit=1;cursor=value",
		"?cursor=not_base64!",
		"?cursor=" + strings.Repeat("a", audit.MaxCursorEncodedBytes+1),
		"?cursor=first&cursor=second",
		"?action=",
		"?action=unknown.action",
		"?action=auth.login.failed&action=auth.login.succeeded",
		"?correlation_id=one&correlation_id=two",
		"?correlation_id=" + strings.Repeat("c", audit.MaxCorrelationIdentityBytes+1),
		"?object_type=user",
		"?object_id=user-1",
		"?object_type=unknown&object_id=user-1",
		"?object_type=user&object_id=" + strings.Repeat("u", audit.MaxIdentityBytes+1),
		"?action=auth.login.failed&object_type=device&object_id=device-1",
	}
	for _, rawQuery := range queries {
		t.Run(rawQuery, func(t *testing.T) {
			var reads atomic.Int32
			server := newAuditTestServer(
				auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
					reads.Add(1)
					return audit.Page{}, nil
				}),
				allowAuditReads(),
			)
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events"+rawQuery, "session"))

			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_request") {
				t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
			}
			if reads.Load() != 0 {
				t.Fatalf("reader calls = %d, want zero", reads.Load())
			}
		})
	}
}

func TestAuditReadReauthorizesBeforeParsingEveryPage(t *testing.T) {
	var authorizations atomic.Int32
	var reads atomic.Int32
	server := newAuditTestServer(
		auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
			reads.Add(1)
			return audit.Page{}, nil
		}),
		WithAuditAuthorizer(AuditAuthorizerFunc(func(context.Context, string) error {
			if authorizations.Add(1) == 1 {
				return nil
			}
			return errors.New("session revoked")
		})),
	)

	first := httptest.NewRecorder()
	server.Handler().ServeHTTP(first, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))
	second := httptest.NewRecorder()
	server.Handler().ServeHTTP(second, newAuditRequest(http.MethodGet, "/v1/audit/events?cursor=malformed!", "session"))

	if first.Code != http.StatusOK || second.Code != http.StatusUnauthorized {
		t.Fatalf("page statuses = first:%d second:%d", first.Code, second.Code)
	}
	if authorizations.Load() != 2 || reads.Load() != 1 {
		t.Fatalf("calls = authorizer:%d reader:%d", authorizations.Load(), reads.Load())
	}
}

func TestAuditCollectionMutationMethodsReturn405WithoutAuthorizationOrRead(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var authorizations atomic.Int32
			var reads atomic.Int32
			server := newAuditTestServer(
				auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
					reads.Add(1)
					return audit.Page{}, nil
				}),
				WithAuditAuthorizer(AuditAuthorizerFunc(func(context.Context, string) error {
					authorizations.Add(1)
					return nil
				})),
			)
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, newAuditRequest(method, "/v1/audit/events", "session"))

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s status = %d body = %q", method, response.Code, response.Body.String())
			}
			if !strings.Contains(response.Header().Get("Allow"), http.MethodGet) {
				t.Fatalf("%s Allow = %q", method, response.Header().Get("Allow"))
			}
			if authorizations.Load() != 0 || reads.Load() != 0 {
				t.Fatalf("calls = authorizer:%d reader:%d, want zero", authorizations.Load(), reads.Load())
			}
		})
	}
}

func TestAuditReadFailureIsSanitized(t *testing.T) {
	const secret = "postgres://guardian:do-not-leak@database/guardian"
	server := newAuditTestServer(
		auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
			return audit.Page{}, errors.New(secret)
		}),
		allowAuditReads(),
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestAuditReadRejectsInvalidReaderPage(t *testing.T) {
	server := newAuditTestServer(
		auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
			return audit.Page{Records: []audit.Record{{Sequence: 0}}}, nil
		}),
		allowAuditReads(),
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), "internal_error") {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestNilLoggerFallsBackSafely(t *testing.T) {
	server := NewServer(
		"127.0.0.1:0",
		readinessFunc(func(context.Context) error { return nil }),
		nil,
		WithAuditReader(auditReaderFunc(func(context.Context, audit.ListQuery) (audit.Page, error) {
			return audit.Page{}, errors.New("deliberate failure")
		})),
		allowAuditReads(),
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestAuthorizedAuditReadWithoutReaderFailsClosed(t *testing.T) {
	server := NewServer(
		"127.0.0.1:0",
		readinessFunc(func(context.Context) error { return nil }),
		discardLogger(),
		allowAuditReads(),
	)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, newAuditRequest(http.MethodGet, "/v1/audit/events", "session"))

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "audit_unavailable") {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func newAuditTestServer(reader audit.Reader, options ...Option) *Server {
	allOptions := append([]Option{WithAuditReader(reader)}, options...)
	return NewServer(
		"127.0.0.1:0",
		readinessFunc(func(context.Context) error { return nil }),
		discardLogger(),
		allOptions...,
	)
}

func newAuditRequest(method, target, session string) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	if session != "" {
		request.AddCookie(&http.Cookie{Name: auditSessionCookieName, Value: session})
	}
	return request
}

func allowAuditReads() Option {
	return WithAuditAuthorizer(AuditAuthorizerFunc(func(context.Context, string) error { return nil }))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
