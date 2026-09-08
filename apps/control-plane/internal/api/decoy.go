package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/deception"
)

// DecoyService is the P2-W15 application boundary exposed over HTTP. It has no
// observed-state write: observed truth arrives over the device channel, and no
// operator request can assert it.
type DecoyService interface {
	ListDecoys(context.Context, string, int32) ([]deception.View, error)
	Decoy(context.Context, string, string) (deception.View, error)
	CreateDecoy(context.Context, string, deception.Input, deception.Mutation) (deception.Decoy, error)
	UpdateDecoy(context.Context, string, string, deception.Input, int64, deception.Mutation) (deception.Decoy, error)
	EnableDecoy(context.Context, string, string, int64, deception.Mutation) (deception.Decoy, error)
	DisableDecoy(context.Context, string, string, int64, deception.Mutation) (deception.Decoy, error)
	RemoveDecoy(context.Context, string, string, int64, deception.Mutation) error
}

func WithDecoyService(service DecoyService) Option {
	return func(server *Server) { server.decoyService = service }
}

// decoyWriteRequest is the complete set of fields an operator may supply. There
// is deliberately no image, command, argument, mount, capability, port, or
// digest field: runtime detail is resolved from the server-side pack index by
// (pack, pack_version), so the API surface has nowhere to put one.
type decoyWriteRequest struct {
	ZoneID      string `json:"zone_id"`
	DisplayName string `json:"display_name"`
	Family      string `json:"family"`
	Persona     string `json:"persona"`
	Address     string `json:"address"`
	Pack        string `json:"pack"`
	PackVersion string `json:"pack_version"`
}

func (r decoyWriteRequest) input() deception.Input {
	return deception.Input{
		ZoneID: r.ZoneID, DisplayName: r.DisplayName, Family: r.Family,
		Persona: r.Persona, Address: r.Address, Pack: r.Pack, PackVersion: r.PackVersion,
	}
}

func (s *Server) handleListDecoys(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	limit, err := parseEnvironmentListLimit(request.URL)
	if err != nil {
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: codeDecoyRequestInvalid})
		return
	}
	items, err := s.decoyService.ListDecoys(request.Context(), request.PathValue("environmentId"), limit)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"decoys": decoyViews(items)})
}

func (s *Server) handleGetDecoy(writer http.ResponseWriter, request *http.Request) {
	if _, ok := s.authorizeEnvironment(writer, request, false); !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	item, err := s.decoyService.Decoy(
		request.Context(), request.PathValue("environmentId"), request.PathValue("decoyId"),
	)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	writeRevisionJSON(writer, http.StatusOK, item.Decoy.Revision, decoyView(item))
}

func (s *Server) handleCreateDecoy(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.decoyMutation(writer, request, actor)
	if !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	var input decoyWriteRequest
	if !s.decodeEnvironmentJSON(writer, request, resourceDecoy, &input) {
		return
	}
	item, err := s.decoyService.CreateDecoy(
		request.Context(), request.PathValue("environmentId"), input.input(), mutation,
	)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	// A newly created decoy has never been reported on, so its observed state
	// is unknown. The response says so rather than implying deployment.
	writeRevisionJSON(writer, http.StatusCreated, item.Revision, decoyView(deception.View{
		Decoy: item, Observed: deception.UnreportedObservation(item.DecoyID, item.CreatedAt),
	}))
}

func (s *Server) handleUpdateDecoy(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.decoyMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceDecoy)
	if !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	var input decoyWriteRequest
	if !s.decodeEnvironmentJSON(writer, request, resourceDecoy, &input) {
		return
	}
	item, err := s.decoyService.UpdateDecoy(
		request.Context(), request.PathValue("environmentId"), request.PathValue("decoyId"),
		input.input(), revision, mutation,
	)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	s.writeDecoyAfterWrite(writer, request, item)
}

func (s *Server) handleEnableDecoy(writer http.ResponseWriter, request *http.Request) {
	s.handleDecoyTransition(writer, request, true)
}

func (s *Server) handleDisableDecoy(writer http.ResponseWriter, request *http.Request) {
	s.handleDecoyTransition(writer, request, false)
}

// handleDecoyTransition serves enable and disable, which are separate
// operations rather than one PATCH field because WC-D16 assigns confirmation
// levels to operations rather than to fields inside one.
func (s *Server) handleDecoyTransition(writer http.ResponseWriter, request *http.Request, enable bool) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.decoyMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceDecoy)
	if !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	transition := s.decoyService.DisableDecoy
	if enable {
		transition = s.decoyService.EnableDecoy
	}
	item, err := transition(
		request.Context(), request.PathValue("environmentId"), request.PathValue("decoyId"),
		revision, mutation,
	)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	s.writeDecoyAfterWrite(writer, request, item)
}

func (s *Server) handleDeleteDecoy(writer http.ResponseWriter, request *http.Request) {
	actor, ok := s.authorizeEnvironment(writer, request, true)
	if !ok {
		return
	}
	mutation, ok := s.decoyMutation(writer, request, actor)
	if !ok {
		return
	}
	revision, ok := s.requireStrongRevision(writer, request, resourceDecoy)
	if !ok {
		return
	}
	if !s.decoyAvailable(writer, request) {
		return
	}
	err := s.decoyService.RemoveDecoy(
		request.Context(), request.PathValue("environmentId"), request.PathValue("decoyId"),
		revision, mutation,
	)
	if err != nil {
		s.writeDecoyError(writer, request, err)
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusNoContent)
}

// writeDecoyAfterWrite re-reads the decoy so the response carries the observed
// record that already exists. It never synthesises one from the write: a
// successful configuration change says nothing about what the network is doing.
func (s *Server) writeDecoyAfterWrite(writer http.ResponseWriter, request *http.Request, item deception.Decoy) {
	view, err := s.decoyService.Decoy(request.Context(), item.EnvironmentID, item.DecoyID)
	if err != nil {
		view = deception.View{
			Decoy: item, Observed: deception.UnreportedObservation(item.DecoyID, item.UpdatedAt),
		}
	}
	view.Decoy = item
	writeRevisionJSON(writer, http.StatusOK, item.Revision, decoyView(view))
}

// decoyView keeps desired and observed state in two sibling objects. There is
// no combined "state" field anywhere in this response, at any level, because
// the moment one exists a reader will treat it as the truth about coverage.
func decoyView(view deception.View) map[string]any {
	return map[string]any{"decoy": decoyBody(view.Decoy), "observed": observedBody(view.Observed)}
}

func decoyViews(views []deception.View) []map[string]any {
	items := make([]map[string]any, 0, len(views))
	for _, view := range views {
		items = append(items, decoyView(view))
	}
	return items
}

func decoyBody(decoy deception.Decoy) map[string]any {
	body := map[string]any{
		"decoy_id":       decoy.DecoyID,
		"environment_id": decoy.EnvironmentID,
		"zone_id":        decoy.ZoneID,
		// Operator-supplied and therefore untrusted to the renderer. Every other
		// string in this object is a closed token the console may treat as a
		// category; this one is not.
		"display_name":      decoy.DisplayName,
		"family":            decoy.Family,
		"persona":           decoy.Persona,
		"interaction_level": decoy.InteractionLevel,
		"address":           decoy.Address,
		"pack":              decoy.Pack,
		"pack_version":      decoy.PackVersion,
		"pack_digest":       nil,
		"desired_state":     decoy.DesiredState,
		"revision":          decoy.Revision,
		"created_at":        decoy.CreatedAt,
		"updated_at":        decoy.UpdatedAt,
	}
	if decoy.PackDigest != "" {
		body["pack_digest"] = decoy.PackDigest
	}
	return body
}

func observedBody(observed deception.Observation) map[string]any {
	conditions := make([]map[string]any, 0, len(observed.Conditions))
	for _, condition := range observed.Conditions {
		entry := map[string]any{
			"type":                 condition.Type,
			"status":               condition.Status,
			"reason":               condition.Reason,
			"message":              condition.Message,
			"observed_revision":    nil,
			"last_transition_time": condition.LastTransitionTime,
		}
		if condition.ObservedRevision != nil {
			entry["observed_revision"] = *condition.ObservedRevision
		}
		conditions = append(conditions, entry)
	}
	body := map[string]any{
		"observed_state":      observed.State,
		"reporting_device_id": nil,
		"reported_at":         nil,
		"last_interaction_at": nil,
		"desired_revision":    nil,
		"conditions":          conditions,
	}
	if observed.ReportingDeviceID != "" {
		body["reporting_device_id"] = observed.ReportingDeviceID
	}
	if observed.ReportedAt != nil {
		body["reported_at"] = *observed.ReportedAt
	}
	if observed.LastInteractionAt != nil {
		body["last_interaction_at"] = *observed.LastInteractionAt
	}
	if observed.DesiredRevision != nil {
		body["desired_revision"] = *observed.DesiredRevision
	}
	return body
}

func (s *Server) decoyAvailable(writer http.ResponseWriter, request *http.Request) bool {
	if s.decoyService == nil {
		s.writeError(writer, request, http.StatusServiceUnavailable, "decoy_service_unavailable",
			errorDetail{code: codeDecoyUnavailable, retryAfter: environmentUnavailableRetryAfter})
		return false
	}
	return true
}

// writeDecoyError maps one domain error onto the CP-0004 contract. Every HTTP
// status and every `status` slug is exactly what P2-W15 returned; `code` and
// `field_errors` are added beside them.
//
// This is the reason change proposal 0004 exists: the decoy form has seven
// interdependent fields, and a single slug could not say which one was refused.
// The attribution comes from the domain validator that made the decision or
// from a conflict sentinel that names one field by construction. Nothing here
// reads the request body, so no submitted value can reach the response.
func (s *Server) writeDecoyError(writer http.ResponseWriter, request *http.Request, err error) {
	violation, hasField := deception.ViolationOf(err)
	fields := func(field fieldPath, reason fieldReason) []fieldError {
		return []fieldError{{Field: field, Code: reason}}
	}
	switch {
	case errors.Is(err, deception.ErrAddressOutsideZone):
		s.writeError(writer, request, http.StatusBadRequest, "address_outside_zone",
			errorDetail{
				code:   codeDecoyAddressOutsideZone,
				fields: fields("address", "outside_zone"),
			})
	case errors.Is(err, deception.ErrUnknownPack):
		s.writeError(writer, request, http.StatusBadRequest, "unknown_pack",
			errorDetail{
				code:   codeDecoyPackUnknown,
				fields: fields("pack_version", "unknown"),
			})
	case errors.Is(err, deception.ErrDecoyBudgetExhausted):
		// The environment is full. No single field is wrong, so none is named:
		// marking one would tell the operator to change something that is fine.
		s.writeError(writer, request, http.StatusConflict, "decoy_budget_exhausted",
			errorDetail{code: codeDecoyBudgetExhausted})
	case errors.Is(err, deception.ErrInvalidInput):
		var attributed []fieldError
		if hasField {
			attributed = fields(fieldPath(violation.Field), fieldReason(violation.Reason))
		}
		s.writeError(writer, request, http.StatusBadRequest, "invalid_request",
			errorDetail{code: codeDecoyRequestInvalid, fields: attributed})
	case errors.Is(err, deception.ErrNotFound):
		// A zone reference the operator supplied in the body is distinguishable
		// from a decoy or environment that does not exist, because the domain
		// attributed the first to `zone_id` and cannot attribute the others.
		if hasField && violation.Field == deception.FieldZoneID {
			s.writeError(writer, request, http.StatusNotFound, "not_found",
				errorDetail{code: codeDecoyZoneNotFound, fields: fields("zone_id", "unknown")})
			return
		}
		s.writeError(writer, request, http.StatusNotFound, "not_found",
			errorDetail{code: codeDecoyNotFound})
	case errors.Is(err, deception.ErrNameConflict):
		s.writeError(writer, request, http.StatusConflict, "name_conflict",
			errorDetail{
				code:   codeDecoyNameConflicting,
				fields: fields("display_name", "conflicting"),
			})
	case errors.Is(err, deception.ErrAddressConflict):
		s.writeError(writer, request, http.StatusConflict, "address_conflict",
			errorDetail{
				code:   codeDecoyAddressConflicting,
				fields: fields("address", "conflicting"),
			})
	case errors.Is(err, deception.ErrPreconditionFailed):
		s.writeError(writer, request, http.StatusPreconditionFailed, "precondition_failed",
			errorDetail{code: codeDecoyRevisionStale})
	default:
		s.writeError(writer, request, http.StatusInternalServerError, "internal_error",
			errorDetail{code: codeInternalUnexpected})
	}
}

// decoyMutation reuses the environment request-identity rules so one mutation
// header contract covers the whole owner API.
func (s *Server) decoyMutation(
	writer http.ResponseWriter,
	request *http.Request,
	actor string,
) (deception.Mutation, bool) {
	mutation, ok := s.environmentMutation(writer, request, resourceDecoy, actor)
	if !ok {
		return deception.Mutation{}, false
	}
	return deception.Mutation{ActorID: mutation.ActorID, RequestID: mutation.RequestID}, true
}
