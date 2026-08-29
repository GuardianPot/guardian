package health

import (
	"context"
	"errors"
	"time"
)

type EvidenceSource interface {
	ProbeHealth(context.Context) (readOK, writeOK, integrityOK bool)
	HealthSpoolStats(context.Context) (usedBytes int64, directory string, err error)
	HealthReconciliation(context.Context) (desired, observed uint64, state string, found bool, err error)
	HealthIdentity(context.Context) (notAfter time.Time, enrollmentStatus string, found bool, err error)
}

type HelperSnapshot struct {
	Available        bool
	Reason           string
	RuntimeObserved  bool
	RuntimeAvailable bool
	RuntimeTimedOut  bool
}

type HelperEvidence interface {
	HealthSnapshot() HelperSnapshot
}

type EvidenceCollector struct {
	source        EvidenceSource
	helper        HelperEvidence
	spoolCapacity int64
}

func NewEvidenceCollector(source EvidenceSource, helper HelperEvidence, spoolCapacity int64) (*EvidenceCollector, error) {
	if source == nil || helper == nil || spoolCapacity <= 0 {
		return nil, errors.New("health evidence source, helper, and spool capacity are required")
	}
	return &EvidenceCollector{source: source, helper: helper, spoolCapacity: spoolCapacity}, nil
}

func (c *EvidenceCollector) Collect(ctx context.Context, now time.Time) ([]Observation, error) {
	clockAvailable, synchronized, offset := clockMeasurement()
	clock := EvaluateClock(clockAvailable, synchronized, offset)
	observations := make([]Observation, 0, conditionCount-1)
	notAfter, enrollmentStatus, identityFound, err := c.source.HealthIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if !identityFound {
		observations = append(observations, Observation{Type: TypeDeviceCertificateReady, Status: StatusUnknown, Reason: "not_observed"})
	} else {
		certificate, evaluateErr := EvaluateCertificate(now, notAfter, enrollmentStatus == "revoked", false, clock.Status == StatusTrue)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		observations = append(observations, certificate)
	}
	desired, observed, reconciliationState, found, err := c.source.HealthReconciliation(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		observations = append(observations, Observation{Type: TypeConfigConverged, Status: StatusUnknown, Reason: "not_observed"})
	} else {
		revision := observed
		configuration := Observation{Type: TypeConfigConverged, Status: StatusFalse, Reason: "revision_drift", ObservedRevision: &revision}
		if desired == observed && reconciliationState == "converged" {
			configuration.Status, configuration.Reason = StatusTrue, "converged"
		}
		observations = append(observations, configuration)
	}
	readOK, writeOK, integrityOK := c.source.ProbeHealth(ctx)
	observations = append(observations, EvaluateDatabase(true, readOK, writeOK, integrityOK))
	usedBytes, spoolDirectory, err := c.source.HealthSpoolStats(ctx)
	if err != nil {
		observations = append(observations, Observation{Type: TypeSpoolHealthy, Status: StatusUnknown, Reason: "measurement_unavailable"})
	} else {
		freePercent, available := filesystemFreePercent(spoolDirectory)
		spool, evaluateErr := EvaluateSpool(available, usedBytes, c.spoolCapacity, freePercent)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
		observations = append(observations, spool)
	}
	observations = append(observations, clock)
	helper := c.helper.HealthSnapshot()
	runtime := Observation{Type: TypeContainerRuntimeReachable, Status: StatusUnknown, Reason: "not_observed"}
	if helper.RuntimeObserved {
		var evaluateErr error
		runtime, evaluateErr = EvaluateProbe(TypeContainerRuntimeReachable, helper.RuntimeAvailable, helper.RuntimeTimedOut)
		if evaluateErr != nil {
			return nil, evaluateErr
		}
	}
	observations = append(observations, runtime)
	helperObservation := Observation{Type: TypePrivilegedHelperReachable, Status: StatusFalse, Reason: helper.Reason}
	if helper.Available {
		helperObservation.Status, helperObservation.Reason = StatusTrue, "reachable"
	} else if helper.Reason == "" {
		helperObservation.Status, helperObservation.Reason = StatusUnknown, "not_observed"
	}
	if err := (&Condition{Type: helperObservation.Type, Status: helperObservation.Status, Reason: helperObservation.Reason, LastTransitionTime: now}).Validate(); err != nil {
		helperObservation.Status, helperObservation.Reason = StatusFalse, "rpc_unavailable"
	}
	observations = append(observations, helperObservation)
	return observations, nil
}
