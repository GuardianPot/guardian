package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
)

const (
	auditSessionCookieName  = "__Host-guardian_session"
	defaultAuditPageLimit   = audit.DefaultListLimit
	maximumAuditPageLimit   = audit.MaxListLimit
	maximumAuditCursorSize  = audit.MaxCursorEncodedBytes
	maximumAuditSessionSize = 512
)

var errAuditAuthenticationRequired = errors.New("audit authentication is required")

// AuditAuthorizer validates an opaque browser session before an audit read.
// Implementations must not log or persist the supplied session value.
type AuditAuthorizer interface {
	AuthorizeAuditRead(context.Context, string) error
}

// AuditAuthorizerFunc adapts a function to AuditAuthorizer.
type AuditAuthorizerFunc func(context.Context, string) error

// AuthorizeAuditRead calls fn with the request context and opaque session.
func (fn AuditAuthorizerFunc) AuthorizeAuditRead(ctx context.Context, session string) error {
	if fn == nil {
		return errAuditAuthenticationRequired
	}
	return fn(ctx, session)
}

type denyAuditAuthorizer struct{}

func (denyAuditAuthorizer) AuthorizeAuditRead(context.Context, string) error {
	return errAuditAuthenticationRequired
}

// Option configures an optional API dependency while preserving the production
// constructor used by the Control Plane composition root.
type Option func(*Server)

// WithAuditReader overrides the audit reader discovered from Readiness.
func WithAuditReader(reader audit.Reader) Option {
	return func(server *Server) {
		server.auditReader = reader
	}
}

// WithAuditAuthorizer supplies the P1-W2 authentication integration seam.
// A nil authorizer leaves the server fail-closed.
func WithAuditAuthorizer(authorizer AuditAuthorizer) Option {
	return func(server *Server) {
		if authorizer != nil {
			server.auditAuthorizer = authorizer
		}
	}
}

func (s *Server) handleListAuditEvents(writer http.ResponseWriter, request *http.Request) {
	session, ok := singleAuditSession(request)
	if !ok || s.auditAuthorizer == nil || s.auditAuthorizer.AuthorizeAuditRead(request.Context(), session) != nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return
	}

	query, err := parseAuditListQuery(request)
	if err != nil {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return
	}
	if s.auditReader == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "audit_unavailable")
		return
	}

	page, err := s.auditReader.List(request.Context(), query)
	if err != nil {
		s.logger.ErrorContext(request.Context(), "audit read failed")
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := page.Validate(); err != nil {
		s.logger.ErrorContext(request.Context(), "audit reader returned an invalid page")
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
		return
	}
	response := auditPageResponse{
		Events:     make([]auditEventResponse, 0, len(page.Records)),
		NextCursor: page.NextCursor,
	}
	for _, record := range page.Records {
		response.Events = append(response.Events, newAuditEventResponse(record))
	}
	writeJSON(writer, http.StatusOK, response)
}

func singleAuditSession(request *http.Request) (string, bool) {
	cookies := request.CookiesNamed(auditSessionCookieName)
	if len(cookies) != 1 || cookies[0].Value == "" || len(cookies[0].Value) > maximumAuditSessionSize ||
		strings.TrimSpace(cookies[0].Value) == "" {
		return "", false
	}
	return cookies[0].Value, true
}

func parseAuditListQuery(request *http.Request) (audit.ListQuery, error) {
	values, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return audit.ListQuery{}, errors.New("invalid audit query encoding")
	}
	allowed := map[string]struct{}{
		"action":         {},
		"correlation_id": {},
		"cursor":         {},
		"limit":          {},
		"object_id":      {},
		"object_type":    {},
	}
	for name, entries := range values {
		if _, ok := allowed[name]; !ok || len(entries) != 1 || entries[0] == "" {
			return audit.ListQuery{}, errors.New("invalid audit query parameter")
		}
	}

	query := audit.ListQuery{
		Limit:         defaultAuditPageLimit,
		Action:        audit.Action(values.Get("action")),
		CorrelationID: values.Get("correlation_id"),
		ObjectType:    audit.ObjectType(values.Get("object_type")),
		ObjectID:      values.Get("object_id"),
	}
	if rawLimit := values.Get("limit"); rawLimit != "" {
		limit, err := strconv.ParseInt(rawLimit, 10, 32)
		if err != nil || limit < 1 || limit > int64(maximumAuditPageLimit) {
			return audit.ListQuery{}, errors.New("invalid audit page limit")
		}
		query.Limit = int32(limit)
	}
	if rawCursor := values.Get("cursor"); rawCursor != "" {
		if len(rawCursor) > maximumAuditCursorSize {
			return audit.ListQuery{}, errors.New("invalid audit cursor")
		}
		cursor, err := audit.ParseCursor(rawCursor)
		if err != nil {
			return audit.ListQuery{}, errors.New("invalid audit cursor")
		}
		query.Cursor = cursor
	}
	if err := query.Validate(); err != nil {
		return audit.ListQuery{}, errors.New("invalid audit filters")
	}
	return query, nil
}

type auditPageResponse struct {
	Events     []auditEventResponse `json:"events"`
	NextCursor string               `json:"next_cursor,omitempty"`
}

type auditEventResponse struct {
	EventID       string          `json:"event_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	RecordedAt    time.Time       `json:"recorded_at"`
	Actor         auditActorDTO   `json:"actor"`
	Action        audit.Action    `json:"action"`
	Object        auditObjectDTO  `json:"object"`
	CorrelationID string          `json:"correlation_id"`
	RequestID     string          `json:"request_id,omitempty"`
	Before        *audit.Snapshot `json:"before,omitempty"`
	After         *audit.Snapshot `json:"after,omitempty"`
}

type auditActorDTO struct {
	Type audit.ActorType `json:"type"`
	ID   string          `json:"id"`
}

type auditObjectDTO struct {
	Type audit.ObjectType `json:"type"`
	ID   string           `json:"id"`
}

func newAuditEventResponse(record audit.Record) auditEventResponse {
	return auditEventResponse{
		EventID:       record.EventID,
		OccurredAt:    record.Event.OccurredAt,
		RecordedAt:    record.RecordedAt,
		Actor:         auditActorDTO{Type: record.Event.Actor.Type, ID: record.Event.Actor.ID},
		Action:        record.Event.Action,
		Object:        auditObjectDTO{Type: record.Event.Object.Type, ID: record.Event.Object.ID},
		CorrelationID: record.Event.CorrelationID,
		RequestID:     record.Event.RequestID,
		Before:        record.Event.Before,
		After:         record.Event.After,
	}
}

func writeJSON(writer http.ResponseWriter, code int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(code)
	_, _ = writer.Write(append(payload, '\n'))
}
