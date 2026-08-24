package audit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func validTestEvent(t *testing.T) Event {
	t.Helper()
	snapshot, err := NewSnapshot(map[string]any{"enabled": true})
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	return Event{
		OccurredAt: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		Actor: Actor{
			Type: ActorTypeUser,
			ID:   "user-1",
		},
		Action: ActionSecuritySettingChanged,
		Object: ObjectRef{
			Type: ObjectTypeSecuritySetting,
			ID:   "setting-1",
		},
		CorrelationID: "correlation-1",
		RequestID:     "request-1",
		After:         &snapshot,
	}
}

func TestEventValidate(t *testing.T) {
	event := validTestEvent(t)
	if err := event.Validate(); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	tests := map[string]func(*Event){
		"occurred at":        func(e *Event) { e.OccurredAt = time.Time{} },
		"actor type":         func(e *Event) { e.Actor.Type = "admin" },
		"actor ID":           func(e *Event) { e.Actor.ID = "" },
		"actor ID bound":     func(e *Event) { e.Actor.ID = strings.Repeat("a", MaxIdentityBytes+1) },
		"action":             func(e *Event) { e.Action = "incident.created" },
		"action object pair": func(e *Event) { e.Object.Type = ObjectTypeUser },
		"object ID":          func(e *Event) { e.Object.ID = " object " },
		"correlation":        func(e *Event) { e.CorrelationID = "" },
		"correlation bound":  func(e *Event) { e.CorrelationID = strings.Repeat("c", MaxCorrelationIdentityBytes+1) },
		"request control":    func(e *Event) { e.RequestID = "request\nvalue" },
		"request bound":      func(e *Event) { e.RequestID = strings.Repeat("r", MaxCorrelationIdentityBytes+1) },
		"zero snapshot":      func(e *Event) { e.Before = &Snapshot{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := event
			mutate(&candidate)
			if err := candidate.Validate(); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("error = %v, want ErrInvalidEvent", err)
			}
		})
	}
}

func TestAuditTimestampValidation(t *testing.T) {
	plusOneHour := time.FixedZone("plus-one", 60*60)
	minusOneHour := time.FixedZone("minus-one", -60*60)
	invalidJSONOffset := time.FixedZone("invalid-plus-twenty-four", 24*60*60)

	if _, err := time.Date(2026, 8, 24, 12, 0, 0, 0, invalidJSONOffset).MarshalJSON(); err == nil {
		t.Fatal("test setup: a +24:00 timestamp must not be RFC3339 JSON serializable")
	}

	tests := []struct {
		name      string
		timestamp time.Time
		valid     bool
	}{
		{
			name: "minimum persisted UTC instant through positive offset",
			timestamp: time.Date(MinAuditTimestampYear, 1, 1, 1, 0, 0, 0, plusOneHour).
				Add(AuditTimestampPrecision),
			valid: true,
		},
		{
			name:      "maximum UTC instant",
			timestamp: time.Date(MaxAuditTimestampYear, 12, 31, 23, 59, 59, 999_999_999, time.UTC),
			valid:     true,
		},
		{
			name:      "Go zero time",
			timestamp: time.Time{},
		},
		{
			name:      "Go zero plus one nanosecond loses precision",
			timestamp: time.Time{}.Add(time.Nanosecond),
		},
		{
			name:      "Go zero plus less than one microsecond loses precision",
			timestamp: time.Time{}.Add(AuditTimestampPrecision - time.Nanosecond),
		},
		{
			name:      "UTC year zero",
			timestamp: time.Date(0, 12, 31, 23, 59, 59, 999_999_999, time.UTC),
		},
		{
			name:      "UTC year ten thousand",
			timestamp: time.Date(MaxAuditTimestampYear+1, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "local year one normalizes into UTC year zero",
			timestamp: time.Date(MinAuditTimestampYear, 1, 1, 0, 30, 0, 0, plusOneHour),
		},
		{
			name:      "local year nine thousand nine hundred ninety-nine normalizes into UTC year ten thousand",
			timestamp: time.Date(MaxAuditTimestampYear, 12, 31, 23, 30, 0, 0, minusOneHour),
		},
		{
			name:      "RFC3339-invalid custom offset",
			timestamp: time.Date(2026, 8, 24, 12, 0, 0, 0, invalidJSONOffset),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := validTestEvent(t)
			event.OccurredAt = test.timestamp
			eventErr := event.Validate()

			record := Record{
				EventID:    "018f90d4-4a2b-7cc1-8f31-123456789abc",
				Sequence:   42,
				RecordedAt: test.timestamp,
				Event:      validTestEvent(t),
			}
			recordErr := record.Validate()

			if test.valid {
				if eventErr != nil {
					t.Fatalf("Event.Validate error = %v", eventErr)
				}
				if recordErr != nil {
					t.Fatalf("Record.Validate error = %v", recordErr)
				}
				return
			}

			if !errors.Is(eventErr, ErrInvalidEvent) || !errors.Is(eventErr, ErrInvalidTimestamp) {
				t.Errorf("Event.Validate error = %v, want ErrInvalidEvent and ErrInvalidTimestamp", eventErr)
			}
			if !errors.Is(recordErr, ErrInvalidRecord) || !errors.Is(recordErr, ErrInvalidTimestamp) {
				t.Errorf("Record.Validate error = %v, want ErrInvalidRecord and ErrInvalidTimestamp", recordErr)
			}
		})
	}
}

func TestRecordValidate(t *testing.T) {
	record := Record{
		EventID:    "018f90d4-4a2b-7cc1-8f31-123456789abc",
		Sequence:   42,
		RecordedAt: time.Date(2026, 8, 24, 12, 0, 1, 0, time.UTC),
		Event:      validTestEvent(t),
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}

	invalid := []Record{record, record, record}
	invalid[0].EventID = "018f90d4-4a2b-6cc1-8f31-123456789abc"
	invalid[1].Sequence = 0
	invalid[2].RecordedAt = time.Time{}
	for i := range invalid {
		if err := invalid[i].Validate(); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("invalid record %d error = %v", i, err)
		}
	}
}

func TestListQueryValidationAndNormalization(t *testing.T) {
	query := ListQuery{
		Action:        ActionLoginFailed,
		CorrelationID: "correlation-1",
		ObjectType:    ObjectTypeUser,
		ObjectID:      "user-1",
	}
	if err := query.Validate(); err != nil {
		t.Fatalf("valid query rejected: %v", err)
	}
	normalized, err := query.Normalized()
	if err != nil {
		t.Fatalf("Normalized: %v", err)
	}
	if normalized.Limit != DefaultListLimit {
		t.Fatalf("normalized limit = %d", normalized.Limit)
	}

	cursor, err := NewCursor(100, 75)
	if err != nil {
		t.Fatal(err)
	}
	query.Cursor = cursor
	query.Limit = MaxListLimit
	if err := query.Validate(); err != nil {
		t.Fatalf("bounded cursor query rejected: %v", err)
	}

	invalid := []ListQuery{
		{Limit: -1},
		{Limit: MaxListLimit + 1},
		{Action: "incident.created"},
		{CorrelationID: strings.Repeat("c", MaxCorrelationIdentityBytes+1)},
		{ObjectType: ObjectTypeUser},
		{ObjectID: "user-1"},
		{ObjectType: ObjectTypeUser, ObjectID: "user-1", Action: ActionLogout},
		{Cursor: Cursor{AnchorSequence: 5}},
	}
	for i, candidate := range invalid {
		if err := candidate.Validate(); !errors.Is(err, ErrInvalidListQuery) {
			t.Errorf("invalid query %d error = %v", i, err)
		}
	}
}

func TestPageValidate(t *testing.T) {
	record := Record{
		EventID:    "018f90d4-4a2b-7cc1-8f31-123456789abc",
		Sequence:   42,
		RecordedAt: time.Date(2026, 8, 24, 12, 0, 1, 0, time.UTC),
		Event:      validTestEvent(t),
	}
	cursor, err := NewCursor(100, 42)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := cursor.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := (Page{Records: []Record{record}, NextCursor: encoded}).Validate(); err != nil {
		t.Fatalf("valid page rejected: %v", err)
	}
	if err := (Page{Records: []Record{record}, NextCursor: "bad"}).Validate(); !errors.Is(err, ErrInvalidPage) {
		t.Fatalf("invalid cursor error = %v", err)
	}
	other := record
	other.Sequence = record.Sequence + 1
	if err := (Page{Records: []Record{record, other}}).Validate(); !errors.Is(err, ErrInvalidPage) {
		t.Fatalf("non-descending page error = %v", err)
	}
	mismatched, err := NewCursor(100, record.Sequence-1)
	if err != nil {
		t.Fatal(err)
	}
	mismatchedEncoded, err := mismatched.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := (Page{Records: []Record{record}, NextCursor: mismatchedEncoded}).Validate(); !errors.Is(err, ErrInvalidPage) {
		t.Fatalf("mismatched cursor error = %v", err)
	}
}

type testContractAdapter struct{}

func (testContractAdapter) Append(context.Context, Event) (Record, error) { return Record{}, nil }
func (testContractAdapter) List(context.Context, ListQuery) (Page, error) { return Page{}, nil }

var _ Appender = testContractAdapter{}
var _ Reader = testContractAdapter{}
