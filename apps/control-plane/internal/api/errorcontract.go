package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
)

// The CP-0004 structured error contract.
//
// `status` keeps every value and every meaning it had in Phase 1. Everything
// added here is optional, so a client that reads only `status` is unaffected
// and an endpoint that has not been migrated still writes exactly
// `{"status": "..."}`. There is no flag day and no compatibility layer.
//
// The three vocabularies below are contract surfaces and are treated the way
// internal/audit/vocabulary.go treats audit actions: closed, enumerated in
// openapi/guardian.yaml, and checked for drift by a test. Nothing here is
// operator-facing prose. A console maps these identifiers to the WCX-08 text
// catalogue, which is where all wording lives and is reviewed.

// errorCode is the closed CP-0004 error-code vocabulary. It is the machine
// distinction the WCX-02 taxonomy previously had to infer from an HTTP status
// code, moved out of convention and into the contract.
type errorCode string

const (
	// codeInternalUnexpected is the only code an unhandled failure may carry. It
	// says nothing about what failed, deliberately.
	codeInternalUnexpected errorCode = "internal.unexpected"

	codeEnvironmentRequestInvalid   errorCode = "environment.request.invalid"
	codeEnvironmentNotFound         errorCode = "environment.not_found"
	codeEnvironmentNameConflicting  errorCode = "environment.display_name.conflicting"
	codeEnvironmentRevisionRequired errorCode = "environment.revision.required"
	codeEnvironmentRevisionStale    errorCode = "environment.revision.stale"
	codeEnvironmentUnavailable      errorCode = "environment.unavailable"
	codeZoneRequestInvalid          errorCode = "zone.request.invalid"
	codeZoneNotFound                errorCode = "zone.not_found"
	codeZoneNameConflicting         errorCode = "zone.display_name.conflicting"
	codeZoneCIDROverlapping         errorCode = "zone.cidr.overlapping"
	codeZoneRevisionRequired        errorCode = "zone.revision.required"
	codeZoneRevisionStale           errorCode = "zone.revision.stale"
	codeDecoyRequestInvalid         errorCode = "decoy.request.invalid"
	codeDecoyNotFound               errorCode = "decoy.not_found"
	codeDecoyZoneNotFound           errorCode = "decoy.zone.not_found"
	codeDecoyNameConflicting        errorCode = "decoy.display_name.conflicting"
	codeDecoyAddressConflicting     errorCode = "decoy.address.conflicting"
	codeDecoyAddressOutsideZone     errorCode = "decoy.address.outside_zone"
	codeDecoyPackUnknown            errorCode = "decoy.pack.unknown"
	codeDecoyBudgetExhausted        errorCode = "decoy.budget.exhausted"
	codeDecoyRevisionRequired       errorCode = "decoy.revision.required"
	codeDecoyRevisionStale          errorCode = "decoy.revision.stale"
	codeDecoyUnavailable            errorCode = "decoy.unavailable"
)

// errorCodes returns the vocabulary in declaration order as a defensive copy.
// The OpenAPI drift test compares this list against the contract enum.
func errorCodes() []errorCode {
	return []errorCode{
		codeInternalUnexpected,
		codeEnvironmentRequestInvalid,
		codeEnvironmentNotFound,
		codeEnvironmentNameConflicting,
		codeEnvironmentRevisionRequired,
		codeEnvironmentRevisionStale,
		codeEnvironmentUnavailable,
		codeZoneRequestInvalid,
		codeZoneNotFound,
		codeZoneNameConflicting,
		codeZoneCIDROverlapping,
		codeZoneRevisionRequired,
		codeZoneRevisionStale,
		codeDecoyRequestInvalid,
		codeDecoyNotFound,
		codeDecoyZoneNotFound,
		codeDecoyNameConflicting,
		codeDecoyAddressConflicting,
		codeDecoyAddressOutsideZone,
		codeDecoyPackUnknown,
		codeDecoyBudgetExhausted,
		codeDecoyRevisionRequired,
		codeDecoyRevisionStale,
		codeDecoyUnavailable,
	}
}

// valid reports membership in the closed vocabulary. An assembled code that is
// not in it is dropped rather than emitted, so a typo in a handler degrades to
// the Phase 1 body instead of shipping an undeclared identifier to the console.
func (c errorCode) valid() bool {
	for _, candidate := range errorCodes() {
		if candidate == c {
			return true
		}
	}
	return false
}

// fieldPath is the closed set of request-body field paths an error may name. It
// is the union of what the environment and decoy domains can attribute, and it
// is a closed set for one reason: a field path reaches the operator's browser,
// and it must be impossible for a submitted value to end up in it.
type fieldPath string

func fieldPaths() []fieldPath {
	return []fieldPath{
		"display_name",
		"cidr",
		"zone_id",
		"family",
		"persona",
		"address",
		"pack",
		"pack_version",
	}
}

func (f fieldPath) valid() bool {
	for _, candidate := range fieldPaths() {
		if candidate == f {
			return true
		}
	}
	return false
}

// fieldReason is the closed set of machine reasons one field may be rejected
// for. It is the union of the environment and decoy domain reason vocabularies.
type fieldReason string

func fieldReasons() []fieldReason {
	return []fieldReason{
		"malformed",
		"out_of_range",
		"unsupported",
		"unknown",
		"outside_zone",
		"conflicting",
	}
}

func (r fieldReason) valid() bool {
	for _, candidate := range fieldReasons() {
		if candidate == r {
			return true
		}
	}
	return false
}

// fieldError is one field-level rejection.
//
// `field` names a request-body field path and never carries a submitted value.
// `code` is a closed machine reason, not a sentence.
//
// `message_key` is declared because change proposal 0004 constraint 3 defines
// what it must be when present — a WCX-08 catalogue key, never a sentence — but
// no handler sets one. The console has no per-code catalogue entry to key
// against yet, so emitting one would assert a key that does not exist. A test
// asserts it stays absent.
type fieldError struct {
	Field      fieldPath   `json:"field"`
	Code       fieldReason `json:"code"`
	MessageKey string      `json:"message_key,omitempty"`
}

func (e fieldError) valid() bool { return e.Field.valid() && e.Code.valid() && e.MessageKey == "" }

// errorBody is the wire shape. `status` is required and unchanged; every other
// field is omitted when unset, so writeStatus still produces `{"status":"..."}`
// byte for byte.
//
// There is deliberately no `message`, `detail`, `title`, or `description` field
// at any level. Adding one would bypass the WCX-08 text catalogue and the
// wording review that catalogue exists to enable.
type errorBody struct {
	Status      string       `json:"status"`
	Code        errorCode    `json:"code,omitempty"`
	FieldErrors []fieldError `json:"field_errors,omitempty"`
	RetryAfter  *int         `json:"retry_after,omitempty"`
	RequestID   string       `json:"request_id,omitempty"`
}

// errorDetail is what a migrated handler supplies. The zero value is a bare
// Phase 1 response, which is what an unmigrated path produces.
type errorDetail struct {
	code   errorCode
	fields []fieldError
	// retryAfter is a hint in whole seconds. Zero means absent.
	retryAfter int
}

// resource names the migrated resource a handler belongs to. It exists because
// the environment, zone, and decoy handlers share validation helpers, and a
// shared helper must still emit the calling resource's code.
type resource string

const (
	resourceEnvironment resource = "environment"
	resourceZone        resource = "zone"
	resourceDecoy       resource = "decoy"
)

// code assembles a vocabulary entry from the resource and a suffix. An
// assembled value outside the closed set fails errorCode.valid and is dropped.
func (r resource) code(suffix string) errorCode { return errorCode(string(r) + "." + suffix) }

// requestIDBytes is the entropy behind one correlation identifier. Sixteen
// random bytes is well past any practical collision or guessing concern for a
// value whose only job is to match a console screen to a log line.
const requestIDBytes = 16

// newRequestID returns an opaque correlation identifier.
//
// It is crypto/rand bytes, hex-encoded, and nothing else. There is no
// timestamp, no counter, no sequence, and no environment, zone, decoy, device,
// session, or actor identity in it, so nothing can be derived from one
// (change proposal 0004, constraint 5). That is also why it is not a ULID: the
// proposal's illustrative example showed one, but a ULID embeds its creation
// time, and the constraint that it carry no meaning is the binding half.
//
// If the system entropy source fails the identifier is empty and the field is
// omitted. A missing identifier costs support a correlation; a predictable one
// would be a contract violation.
func newRequestID() string {
	raw := make([]byte, requestIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	return hex.EncodeToString(raw)
}

// writeStatus writes the Phase 1 body. Every field CP-0004 adds is optional and
// unset here, so an endpoint that has not been migrated is byte-identical to
// what it returned before this change.
func writeStatus(writer http.ResponseWriter, code int, status string) {
	writeErrorBody(writer, code, errorBody{Status: status})
}

// writeError writes a migrated CP-0004 body and logs the correlation
// identifier beside the code so an owner can match a console screen to a
// Control Plane log line. The log line carries the route pattern and the closed
// vocabulary entries only; no operator input reaches it.
func (s *Server) writeError(
	writer http.ResponseWriter,
	request *http.Request,
	httpStatus int,
	status string,
	detail errorDetail,
) {
	body := errorBody{Status: status, RequestID: newRequestID()}
	if detail.code.valid() {
		body.Code = detail.code
	}
	for _, entry := range detail.fields {
		if entry.valid() {
			body.FieldErrors = append(body.FieldErrors, entry)
		}
	}
	if detail.retryAfter > 0 {
		retryAfter := detail.retryAfter
		body.RetryAfter = &retryAfter
	}
	s.logError(request, httpStatus, body)
	writeErrorBody(writer, httpStatus, body)
}

func (s *Server) logError(request *http.Request, httpStatus int, body errorBody) {
	if s == nil || s.logger == nil || request == nil {
		return
	}
	attributes := []any{
		slog.String("request_id", body.RequestID),
		slog.String("status", body.Status),
		slog.String("code", string(body.Code)),
		slog.Int("http_status", httpStatus),
		slog.String("method", request.Method),
		slog.String("route", request.Pattern),
	}
	if httpStatus >= http.StatusInternalServerError {
		s.logger.ErrorContext(request.Context(), "request failed", attributes...)
		return
	}
	s.logger.InfoContext(request.Context(), "request rejected", attributes...)
}

func writeErrorBody(writer http.ResponseWriter, code int, body errorBody) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(code)
	_ = json.NewEncoder(writer).Encode(body)
}
