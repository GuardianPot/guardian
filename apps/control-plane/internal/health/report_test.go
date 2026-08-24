package health

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const testReportID = "01890f8e-7b5d-7cc3-98c4-dc0c0c07398f"

func TestReportRoundTripIsStrictAndDefensive(t *testing.T) {
	report := healthyReport(1, testReportID, testTime)
	encoded, err := MarshalReport(report)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Conditions[0].Message = "changed"
	if report.Conditions[0].Message == "changed" {
		t.Fatal("report parse did not make a defensive condition copy")
	}
	for _, suffix := range []string{` {}`, ` null`} {
		if _, err := ParseReport(append(encoded, suffix...)); !errors.Is(err, ErrInvalidReport) {
			t.Fatalf("trailing value error = %v", err)
		}
	}
	unknown := bytes.Replace(encoded, []byte(`"sequence":1`), []byte(`"unexpected":1,"sequence":1`), 1)
	if _, err := ParseReport(unknown); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestReportRejectsIdentityOrderingAndCompletenessFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Report)
		want   error
	}{
		{"schema", func(report *Report) { report.SchemaVersion = "v2" }, ErrInvalidReport},
		{"uuid-version", func(report *Report) { report.ReportID = "01890f8e-7b5d-6cc3-98c4-dc0c0c07398f" }, ErrInvalidReportID},
		{"uuid-uppercase", func(report *Report) { report.ReportID = strings.ToUpper(testReportID) }, ErrInvalidReportID},
		{"zero-sequence", func(report *Report) { report.Sequence = 0 }, ErrInvalidReport},
		{"missing", func(report *Report) { report.Conditions = report.Conditions[:7] }, ErrIncompleteReport},
		{"duplicate", func(report *Report) { report.Conditions[7] = report.Conditions[6] }, ErrDuplicateCondition},
		{"order", func(report *Report) {
			report.Conditions[0], report.Conditions[1] = report.Conditions[1], report.Conditions[0]
		}, ErrNonCanonicalReport},
		{"future-transition", func(report *Report) {
			report.Conditions[0].LastTransitionTime = report.ObservedAt.Add(TimestampPrecision)
		}, ErrInvalidReport},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := healthyReport(1, testReportID, testTime)
			test.mutate(&report)
			if !errors.Is(report.Validate(), test.want) {
				t.Fatalf("error = %v, want %v", report.Validate(), test.want)
			}
		})
	}
}

func TestParseReportRejectsOversizedAndInvalidUTF8Inputs(t *testing.T) {
	if _, err := ParseReport(bytes.Repeat([]byte{'x'}, MaxReportBytes+1)); !errors.Is(err, ErrOversizedReport) {
		t.Fatalf("oversized error = %v", err)
	}
	if _, err := ParseReport([]byte{0xff}); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("UTF-8 error = %v", err)
	}
}
