package health

import "time"

var testTime = time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC)

func healthyConditions(at time.Time) []Condition {
	reasons := map[ConditionType]string{
		TypeEdgeConnected:             "connected",
		TypeDeviceCertificateReady:    "valid",
		TypeConfigConverged:           "converged",
		TypeLocalDatabaseHealthy:      "ready",
		TypeSpoolHealthy:              "ready",
		TypeClockQuality:              "synchronized",
		TypeContainerRuntimeReachable: "reachable",
		TypePrivilegedHelperReachable: "reachable",
	}
	conditions := make([]Condition, 0, len(conditionTypeOrder))
	for _, conditionType := range conditionTypeOrder {
		conditions = append(conditions, Condition{
			Type:               conditionType,
			Status:             StatusTrue,
			Reason:             reasons[conditionType],
			Message:            "Healthy.",
			LastTransitionTime: at,
		})
	}
	return conditions
}

func healthyReport(sequence uint64, reportID string, at time.Time) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		ReportID:      reportID,
		Sequence:      sequence,
		ObservedAt:    at,
		Conditions:    healthyConditions(at),
	}
}
