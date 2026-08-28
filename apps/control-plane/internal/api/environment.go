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
	if !s.environmentAvailable(writer) {
		return
	}
	organization, err := s.environmentService.Organization(request.Context())
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"organization": organization})
}

func (s *Server) handleListEnvironments(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	limit, err := parseEnvironmentListLimit(request.URL)
	if err != nil {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return
	}
	items, err := s.environmentService.ListEnvironments(request.Context(), limit)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"environments": items})
}

func (s *Server) handleCreateEnvironment(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := environmentMutation(writer, request, actor)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !decodeEnvironmentJSON(writer, request, &input) {
		return
	}
	item, err := s.environmentService.CreateEnvironment(
		request.Context(), input.DisplayName, mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusCreated, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleGetEnvironment(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	item, err := s.environmentService.Environment(request.Context(), request.PathValue("environmentId"))
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleUpdateEnvironment(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := environmentMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := requireStrongRevision(writer, request)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !decodeEnvironmentJSON(writer, request, &input) {
		return
	}
	item, err := s.environmentService.UpdateEnvironment(
		request.Context(), request.PathValue("environmentId"), input.DisplayName, revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"environment": item})
}

func (s *Server) handleListZones(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	limit, err := parseEnvironmentListLimit(request.URL)
	if err != nil {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return
	}
	items, err := s.environmentService.ListZones(request.Context(), request.PathValue("environmentId"), limit)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"zones": items})
}

func (s *Server) handleCreateZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := environmentMutation(writer, request, actor)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
		CIDR        string `json:"cidr"`
	}
	if !decodeEnvironmentJSON(writer, request, &input) {
		return
	}
	item, err := s.environmentService.CreateZone(
		request.Context(), request.PathValue("environmentId"), input.DisplayName, input.CIDR,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusCreated, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleGetZone(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	item, err := s.environmentService.Zone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"),
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleUpdateZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := environmentMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := requireStrongRevision(writer, request)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	var input struct {
		DisplayName string `json:"display_name"`
		CIDR        string `json:"cidr"`
	}
	if !decodeEnvironmentJSON(writer, request, &input) {
		return
	}
	item, err := s.environmentService.UpdateZone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"),
		input.DisplayName, input.CIDR, revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Revision, map[string]any{"zone": item})
}

func (s *Server) handleDeleteZone(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := environmentMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := requireStrongRevision(writer, request)
	if !ok {
		return
	}
	if !s.environmentAvailable(writer) {
		return
	}
	err := s.environmentService.RemoveZone(
		request.Context(), request.PathValue("environmentId"), request.PathValue("zoneId"), revision,
		mutation,
	)
	if err != nil {
		s.writeEnvironmentError(writer, request, err)
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

func (s *Server) environmentAvailable(writer http.ResponseWriter) bool {
	if s.environmentService == nil {
		writeStatus(writer, http.StatusServiceUnavailable, "environment_service_unavailable")
		return false
	}
	return true
}

func (s *Server) writeEnvironmentError(writer http.ResponseWriter, request *http.Request, err error) {
	switch {
	case errors.Is(err, environment.ErrInvalidInput):
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, environment.ErrNotFound):
		writeStatus(writer, http.StatusNotFound, "not_found")
	case errors.Is(err, environment.ErrNameConflict):
		writeStatus(writer, http.StatusConflict, "name_conflict")
	case errors.Is(err, environment.ErrCIDRConflict):
		writeStatus(writer, http.StatusConflict, "cidr_conflict")
	case errors.Is(err, environment.ErrPreconditionFailed):
		writeStatus(writer, http.StatusPreconditionFailed, "precondition_failed")
	default:
		s.logger.ErrorContext(request.Context(), "environment operation failed")
		writeStatus(writer, http.StatusInternalServerError, "internal_error")
	}
}

func decodeEnvironmentJSON(writer http.ResponseWriter, request *http.Request, destination any) bool {
	if contentTypes := request.Header.Values("Content-Type"); len(contentTypes) != 1 || contentTypes[0] != "application/json" {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return false
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maximumEnvironmentRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return false
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

func requireStrongRevision(writer http.ResponseWriter, request *http.Request) (int64, bool) {
	values := request.Header.Values("If-Match")
	if len(values) == 0 {
		writeStatus(writer, http.StatusPreconditionRequired, "precondition_required")
		return 0, false
	}
	if len(values) != 1 {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return 0, false
	}
	value := values[0]
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' || strings.Contains(value, ",") {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return 0, false
	}
	revision, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
	if err != nil || revision < 1 || strconv.FormatInt(revision, 10) != value[1:len(value)-1] {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return 0, false
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

func environmentMutation(writer http.ResponseWriter, request *http.Request, actor string) (environment.Mutation, bool) {
	values := request.Header.Values("X-Request-ID")
	if len(values) == 0 {
		return environment.Mutation{ActorID: actor}, true
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > environment.MaxRequestIDBytes ||
		strings.TrimSpace(values[0]) != values[0] {
		writeStatus(writer, http.StatusBadRequest, "invalid_request")
		return environment.Mutation{}, false
	}
	for _, r := range values[0] {
		if r < 0x20 || r == 0x7f {
			writeStatus(writer, http.StatusBadRequest, "invalid_request")
			return environment.Mutation{}, false
		}
	}
	return environment.Mutation{ActorID: actor, RequestID: values[0]}, true
}
