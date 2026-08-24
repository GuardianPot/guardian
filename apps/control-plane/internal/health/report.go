package health

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidReport      = errors.New("invalid health report")
	ErrInvalidReportID    = errors.New("invalid health report id")
	ErrIncompleteReport   = errors.New("health report is not a full snapshot")
	ErrOversizedReport    = errors.New("health report exceeds encoded limit")
	ErrDuplicateCondition = errors.New("health report contains a duplicate condition")
	ErrNonCanonicalReport = errors.New("health report conditions are not in canonical order")
)

// Report is the complete Phase 1 Edge health snapshot. Device identity is
// deliberately absent because the mTLS channel supplies it.
type Report struct {
	SchemaVersion string      `json:"schema_version"`
	ReportID      string      `json:"report_id"`
	Sequence      uint64      `json:"sequence"`
	ObservedAt    time.Time   `json:"observed_at"`
	Conditions    []Condition `json:"conditions"`
}

// Validate enforces the complete, canonical, bounded report contract.
func (report Report) Validate() error {
	if report.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: schema version", ErrInvalidReport)
	}
	if !validUUIDv7(report.ReportID) {
		return ErrInvalidReportID
	}
	if report.Sequence == 0 {
		return fmt.Errorf("%w: sequence must be positive", ErrInvalidReport)
	}
	if err := ValidateTimestamp(report.ObservedAt); err != nil {
		return fmt.Errorf("%w: observed at: %v", ErrInvalidReport, err)
	}
	if len(report.Conditions) != len(conditionTypeOrder) {
		return ErrIncompleteReport
	}
	seen := make(map[ConditionType]struct{}, len(report.Conditions))
	for index, condition := range report.Conditions {
		if err := condition.Validate(); err != nil {
			return fmt.Errorf("%w: condition %d: %v", ErrInvalidReport, index, err)
		}
		if condition.LastTransitionTime.After(report.ObservedAt) {
			return fmt.Errorf("%w: transition after observation", ErrInvalidReport)
		}
		if _, exists := seen[condition.Type]; exists {
			return ErrDuplicateCondition
		}
		seen[condition.Type] = struct{}{}
		if condition.Type != conditionTypeOrder[index] {
			return ErrNonCanonicalReport
		}
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrInvalidReport, err)
	}
	if len(encoded) > MaxReportBytes {
		return ErrOversizedReport
	}
	return nil
}

// MarshalReport validates and returns deterministic JSON for the canonical
// condition order.
func MarshalReport(report Report) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("encode health report: %w", err)
	}
	return encoded, nil
}

// ParseReport strictly decodes one bounded JSON object. Unknown fields,
// trailing values, invalid UTF-8, and non-canonical condition order fail.
func ParseReport(encoded []byte) (Report, error) {
	if len(encoded) > MaxReportBytes {
		return Report{}, ErrOversizedReport
	}
	if len(bytes.TrimSpace(encoded)) == 0 || !utf8.Valid(encoded) {
		return Report{}, ErrInvalidReport
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var report Report
	if err := decoder.Decode(&report); err != nil {
		return Report{}, fmt.Errorf("%w: decode: %v", ErrInvalidReport, err)
	}
	if err := consumeEOF(decoder); err != nil {
		return Report{}, err
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	return cloneReport(report), nil
}

func consumeEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: trailing JSON value", ErrInvalidReport)
		}
		return fmt.Errorf("%w: trailing data: %v", ErrInvalidReport, err)
	}
	return nil
}

func validUUIDv7(value string) bool {
	if len(value) != 36 || strings.ToLower(value) != value || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '7' {
		return false
	}
	if !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if slices.Contains([]int{8, 13, 18, 23}, index) {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func cloneReport(report Report) Report {
	copyReport := report
	copyReport.Conditions = make([]Condition, len(report.Conditions))
	for index, condition := range report.Conditions {
		copyCondition := condition
		copyCondition.ObservedRevision = cloneRevision(condition.ObservedRevision)
		copyReport.Conditions[index] = copyCondition
	}
	return copyReport
}
