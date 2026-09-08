package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
)

const maximumEnvironmentRequestBytes = 16 << 10

var errEnvironmentAuthorizationRequired = errors.New("environment authorization is required")

// EnvironmentService is the P1-W3 application boundary exposed over HTTP.
type EnvironmentService interface {
	Organization(context.Context) (environment.Organization, error)
	ListEnvironments(context.Context, int32) ([]environment.Environment, error)
	Environment(context.Context, string) (environment.Environment, error)
	CreateEnvironment(context.Context, string, environment.Mutation) (environment.Environment, error)
	UpdateEnvironment(context.Context, string, string, int64, environment.Mutation) (environment.Environment, error)
	ListZones(context.Context, string, int32) ([]environment.Zone, error)
	Zone(context.Context, string, string) (environment.Zone, error)
	CreateZone(context.Context, string, string, string, environment.Mutation) (environment.Zone, error)
	UpdateZone(context.Context, string, string, string, string, int64, environment.Mutation) (environment.Zone, error)
	RemoveZone(context.Context, string, string, int64, environment.Mutation) error
}

// EnvironmentAuthorizer applies the P1-W2 owner session checks. Mutations
// receive the anti-CSRF token and Origin; reads receive empty values.
type EnvironmentAuthorizer interface {
	AuthorizeEnvironment(context.Context, string, string, string, bool) (string, error)
}

type EnvironmentAuthorizerFunc func(context.Context, string, string, string, bool) (string, error)

func (fn EnvironmentAuthorizerFunc) AuthorizeEnvironment(
	ctx context.Context,
	session, csrf, origin string,
	mutation bool,
) (string, error) {
	if fn == nil {
		return "", errEnvironmentAuthorizationRequired
	}
	return fn(ctx, session, csrf, origin, mutation)
}

type denyEnvironmentAuthorizer struct{}

func (denyEnvironmentAuthorizer) AuthorizeEnvironment(context.Context, string, string, string, bool) (string, error) {
	return "", errEnvironmentAuthorizationRequired
}

func WithEnvironmentService(service EnvironmentService) Option {
	return func(server *Server) { server.environmentService = service }
}

func WithEnvironmentAuthorizer(authorizer EnvironmentAuthorizer) Option {
	return func(server *Server) {
		if authorizer != nil {
			server.environmentAuth = authorizer
		}
	}
}

func (s *Server) handleGetOrganization(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	organization, err := s.environmentService.Organization(request.Context())
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceEnvironment, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"organization": organization})
}

func (s *Server) handleListEnvironments(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	limit, err := parseEnvironmentListLimit(request.URL)
	if err != nil {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: codeEnvironmentRequestInvalid})
		return
	}
	items, err := s.environmentService.ListEnvironments(request.Context(), limit)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceEnvironment, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"environments": items})
}

func (s *Server) handleCreateEnvironment(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.environmentMutation(writer, request, resourceEnvironment, actor)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !s.decodeEnvironmentJSON(writer, request, resourceEnvironment, &input) {
		return
	}
	item, err := s.environmentService.CreateEnvironment(
		request.Context(), input.DisplayName, mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceEnvironment, err)
		return
	}
	writeRevisionJSON(writer, http.StatusCreated, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleGetEnvironment(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	item, err := s.environmentService.Environment(request.Context(), request.PathValue("environmentId"))
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceEnvironment, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleUpdateEnvironment(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.environmentMutation(writer, request, resourceEnvironment, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceEnvironment)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !s.decodeEnvironmentJSON(writer, request, resourceEnvironment, &input) {
		return
	}
	item, err := s.environmentService.UpdateEnvironment(
		request.Context(), request.PathValue("environmentId"), input.DisplayName, revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceEnvironment, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleListZones(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	limit, err := parseEnvironmentListLimit(request.URL)
	if err != nil {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: codeZoneRequestInvalid})
		return
	}
	items, err := s.environmentService.ListZones(request.Context(), request.PathValue("environmentId"), limit)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceZone, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"zones": items})
}

func (s *Server) handleCreateZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.environmentMutation(writer, request, resourceZone, actor)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
		CIDR        string `json:"cidr"`
	}
	if !s.decodeEnvironmentJSON(writer, request, resourceZone, &input) {
		return
	}
	item, err := s.environmentService.CreateZone(
		request.Context(), request.PathValue("environmentId"), input.DisplayName, input.CIDR,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceZone, err)
		return
	}
	writeRevisionJSON(writer, http.StatusCreated, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleGetZone(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	item, err := s.environmentService.Zone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"),
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceZone, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleUpdateZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.environmentMutation(writer, request, resourceZone, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceZone)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
		CIDR        string `json:"cidr"`
	}
	if !s.decodeEnvironmentJSON(writer, request, resourceZone, &input) {
		return
	}
	item, err := s.environmentService.UpdateZone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"),
		input.DisplayName, input.CIDR, revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceZone, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleDeleteZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.environmentMutation(writer, request, resourceZone, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceZone)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer, request) {
		return
	}
	err := s.environmentService.RemoveZone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"), revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, resourceZone, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) authorizeEnvironment(writer http.ResponseWriter, request *http.Request, mutation bool) (string, bool) {
	if !s.requireTLS(writer, request) {
		return "", false
	}
	session, ok := authCookie(request)
	if !ok || s.environmentAuth == nil {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	csrf, origin := "", ""
	if mutation {
		csrf, origin = request.Header.Get(csrfHeaderName), request.Header.Get("Origin")
		if len(request.Header.Values(csrfHeaderName)) != 1 || len(request.Header.Values("Origin")) != 1 ||
			len(csrf) != 43 || len(origin) == 0 || len(origin) > 2048 {
			writeStatus(writer, http.StatusUnauthorized, "unauthorized")
			return "", false
		}
	}
	actor, err := s.environmentAuth.AuthorizeEnvironment(request.Context(), session, csrf, origin, mutation)
	if err != nil || actor == "" || len(actor) > 256 || strings.TrimSpace(actor) != actor {
		writeStatus(writer, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return actor, true
}

// environmentUnavailableRetryAfter is the back-off hint on a 503. It is a hint
// and nothing more: the console may retry sooner or later, and the Control
// Plane makes no promise about being ready when it elapses.
const environmentUnavailableRetryAfter = 5

func (s *Server) environmentAvailable(writer http.ResponseWriter, request *http.Request) bool {
	if s.environmentService == nil {
		s.writeError(writer, request, http.StatusServiceUnavailable, "environment_service_unavailable",
			errorDetail{code: codeEnvironmentUnavailable, retryAfter: environmentUnavailableRetryAfter})
		return false
	}
	return true
}

// writeEnvironmentError maps one domain error onto the CP-0004 contract. The
// HTTP status and the `status` slug are exactly what P1-W3 returned; `code` and
// `field_errors` are added beside them.
//
// The field attribution comes from the domain, never from the request: a
// conflict sentinel names exactly one field by construction, and a validation
// failure carries the field the validator rejected. Nothing here reads the
// submitted body.
func (s *Server) writeEnvironmentError(
	writer http.ResponseWriter,
	request *http.Request,
	subject resource,
	err error,
) {
	switch {
	case errors.Is(err, environment.ErrInvalidInput):
		var fields []fieldError
		if violation, ok := environment.ViolationOf(err); ok {
			fields = []fieldError{{
				Field: fieldPath(violation.Field), Code: fieldReason(violation.Reason),
			}}
		}
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: subject.code("request.invalid"), fields: fields})
	case errors.Is(err, environment.ErrNotFound):
		s.writeError(writer, request, http.StatusNotFound, "not_found",
			errorDetail{code: subject.code("not_found")})
	case errors.Is(err, environment.ErrNameConflict):
		s.writeError(writer, request, http.StatusConflict, "name_conflict",
			errorDetail{
				code:   subject.code("display_name.conflicting"),
				fields: []fieldError{{Field: "display_name", Code: "conflicting"}},
			})
	case errors.Is(err, environment.ErrCIDRConflict):
		s.writeError(writer, request, http.StatusConflict, "cidr_conflict",
			errorDetail{
				code:   codeZoneCIDROverlapping,
				fields: []fieldError{{Field: "cidr", Code: "conflicting"}},
			})
	case errors.Is(err, environment.ErrPreconditionFailed):
		s.writeError(writer, request, http.StatusPreconditionFailed, "precondition_failed",
			errorDetail{code: subject.code("revision.stale")})
	default:
		s.writeError(writer, request, http.StatusInternalServerError, "internal_error",
			errorDetail{code: codeInternalUnexpected})
	}
}

func (s *Server) decodeEnvironmentJSON(
	writer http.ResponseWriter,
	request *http.Request,
	subject resource,
	destination any,
) bool {
	reject := func() bool {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: subject.code("request.invalid")})
		return false
	}
	if contentTypes := request.Header.Values("Content-Type"); len(contentTypes) != 1 || contentTypes[0] != "application/json" {
		return reject()
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximumEnvironmentRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	// The decoder error names the offending key, and an unknown key is one the
	// caller chose, so it is never surfaced. A malformed body is a request-shape
	// failure with no field to attribute.
	if err := decoder.Decode(destination); err != nil {
		return reject()
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return reject()
	}
	return true
}

func (s *Server) requireStrongRevision(
	writer http.ResponseWriter,
	request *http.Request,
	subject resource,
) (int64, bool) {
	invalid := func() (int64, bool) {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: subject.code("request.invalid")})
		return 0, false
	}
	values := request.Header.Values("If-Match")
	if len(values) == 0 {
		s.writeError(writer, request, http.StatusPreconditionRequired, "precondition_required",
			errorDetail{code: subject.code("revision.required")})
		return 0, false
	}
	if len(values) != 1 {
		return invalid()
	}
	value := values[0]
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' || strings.Contains(value, ",") {
		return invalid()
	}
	revision, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
	if err != nil || revision < 1 || strconv.FormatInt(revision, 10) != value[1:len(value)-1] {
		return invalid()
	}
	return revision, true
}

func writeRevisionJSON(writer http.ResponseWriter, code int, revision int64, value any) {
	writer.Header().Set("ETag", strconv.Quote(strconv.FormatInt(revision, 10)))
	writeJSON(writer, code, value)
}

func parseEnvironmentListLimit(requestURL *url.URL) (int32, error) {
	values, err := url.ParseQuery(requestURL.RawQuery)
	if err != nil {
		return 0, errors.New("invalid query encoding")
	}
	for name, entries := range values {
		if name != "limit" || len(entries) != 1 || entries[0] == "" {
			return 0, errors.New("invalid query parameter")
		}
	}
	if values.Get("limit") == "" {
		return environment.DefaultListLimit, nil
	}
	limit, err := strconv.ParseInt(values.Get("limit"), 10, 32)
	if err != nil || limit < 1 || limit > int64(environment.MaxListLimit) {
		return 0, errors.New("invalid list limit")
	}
	return int32(limit), nil
}

// environmentMutation reads the caller-supplied X-Request-ID, which is the
// caller's own idempotency key and is unrelated to the CP-0004 `request_id`
// this server generates. The two are deliberately not connected: echoing a
// caller-chosen string back in an error body would put caller input in a field
// the contract promises is opaque and server-generated.
func (s *Server) environmentMutation(
	writer http.ResponseWriter,
	request *http.Request,
	subject resource,
	actor string,
) (environment.Mutation, bool) {
	reject := func() (environment.Mutation, bool) {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: subject.code("request.invalid")})
		return environment.Mutation{}, false
	}
	values := request.Header.Values("X-Request-ID")
	if len(values) == 0 {
		return environment.Mutation{ActorID: actor}, true
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > environment.MaxRequestIDBytes ||
		strings.TrimSpace(values[0]) != values[0] {
		return reject()
	}
	for _, r := range values[0] {
		if r < 0x20 || r == 0x7f {
			return reject()
		}
	}
	return environment.Mutation{ActorID: actor, RequestID: values[0]}, true
}
