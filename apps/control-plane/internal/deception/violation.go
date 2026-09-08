package deception

import (
	"errors"
	"fmt"
)

// Field is the closed set of decoy request-body field paths a rejection may
// name. It is a vocabulary rather than a free string because a field path
// travels to an operator's browser: change proposal 0004 requires that it name
// a field and never echo a submitted value, and a closed set is the only way to
// make that reviewable in one place.
type Field string

const (
	FieldZoneID      Field = "zone_id"
	FieldDisplayName Field = "display_name"
	FieldFamily      Field = "family"
	FieldPersona     Field = "persona"
	FieldAddress     Field = "address"
	FieldPack        Field = "pack"
	FieldPackVersion Field = "pack_version"
)

// Reason is the closed set of machine reasons one field may be rejected for.
// These are identifiers a console maps to its own wording catalogue, never
// sentences: all operator-facing text stays in WCX-08.
type Reason string

const (
	// ReasonMalformed is a value that is not the shape the field accepts.
	ReasonMalformed Reason = "malformed"
	// ReasonOutOfRange is a well-formed value outside the field's bounds.
	ReasonOutOfRange Reason = "out_of_range"
	// ReasonUnsupported is a token outside a closed vocabulary, or one that
	// contradicts another field.
	ReasonUnsupported Reason = "unsupported"
	// ReasonUnknown is a reference to something that does not exist.
	ReasonUnknown Reason = "unknown"
	// ReasonOutsideZone is an address that is not a host address of its zone.
	ReasonOutsideZone Reason = "outside_zone"
	// ReasonConflicting is a value already taken in this environment.
	ReasonConflicting Reason = "conflicting"
)

// FieldViolation attributes one rejection to one request-body field. It carries
// a closed field path and a closed reason and nothing else. In particular it
// never carries the submitted value: echoing a rejected persona, display name,
// or address back into an error response is the leakage path change proposal
// 0004 exists to close, and a type with nowhere to put a value cannot do it by
// accident.
//
// It wraps the package sentinel it stands for, so every existing
// errors.Is(err, ErrInvalidInput) caller keeps working unchanged.
type FieldViolation struct {
	Field  Field
	Reason Reason
	cause  error
}

// Error is assembled from three closed tokens and the sentinel's fixed text, so
// even the log-facing string cannot contain operator input.
func (v *FieldViolation) Error() string {
	return fmt.Sprintf("%s: field %s is %s", v.cause, v.Field, v.Reason)
}

func (v *FieldViolation) Unwrap() error { return v.cause }

// violate builds the sentinel-wrapping violation returned by every validator
// that can name the request-body field it rejected.
func violate(field Field, reason Reason, cause error) error {
	return &FieldViolation{Field: field, Reason: reason, cause: cause}
}

// ViolationOf reports the field attribution of err, if it has one. An error
// without one is a rejection that names no request-body field, such as a stale
// revision or a missing environment, and callers must not invent a field for it.
func ViolationOf(err error) (*FieldViolation, bool) {
	var violation *FieldViolation
	if errors.As(err, &violation) {
		return violation, true
	}
	return nil, false
}

// Fields returns every field path in declaration order as a defensive copy. The
// API error contract checks its own wire vocabulary against this.
func Fields() []Field {
	return []Field{
		FieldZoneID,
		FieldDisplayName,
		FieldFamily,
		FieldPersona,
		FieldAddress,
		FieldPack,
		FieldPackVersion,
	}
}

// Reasons returns every reason in declaration order as a defensive copy.
func Reasons() []Reason {
	return []Reason{
		ReasonMalformed,
		ReasonOutOfRange,
		ReasonUnsupported,
		ReasonUnknown,
		ReasonOutsideZone,
		ReasonConflicting,
	}
}
