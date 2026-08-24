package health

import (
	"errors"
	"math"
	"time"
)

const (
	CertificateRotationWindow = 10 * 24 * time.Hour
	MaximumHealthyClockOffset = 5 * time.Second
	ProbeTimeout              = 2 * time.Second
	SpoolWarningRatio         = 0.80
	SpoolCriticalRatio        = 0.95
	DiskWarningFreePercent    = 10.0
	DiskCriticalFreePercent   = 5.0
)

var ErrInvalidMeasurement = errors.New("invalid health measurement")

func EvaluateCertificate(now, notAfter time.Time, revoked, rotationFailed, clockReliable bool) (Observation, error) {
	observation := Observation{Type: TypeDeviceCertificateReady}
	if err := ValidateTimestamp(now); err != nil {
		return Observation{}, err
	}
	if !clockReliable {
		observation.Status, observation.Reason = StatusUnknown, "clock_unreliable"
		return observation, nil
	}
	if err := ValidateTimestamp(notAfter); err != nil {
		return Observation{}, err
	}
	switch {
	case revoked:
		observation.Status, observation.Reason = StatusFalse, "revoked"
	case rotationFailed:
		observation.Status, observation.Reason = StatusFalse, "rotation_failed"
	case !now.Before(notAfter):
		observation.Status, observation.Reason = StatusFalse, "expired"
	case notAfter.Sub(now) <= CertificateRotationWindow:
		observation.Status, observation.Reason = StatusFalse, "rotation_window"
	default:
		observation.Status, observation.Reason = StatusTrue, "valid"
	}
	return observation, nil
}

func EvaluateClock(measurementAvailable, synchronized bool, offset time.Duration) Observation {
	observation := Observation{Type: TypeClockQuality}
	if !measurementAvailable {
		observation.Status, observation.Reason = StatusUnknown, "measurement_unavailable"
		return observation
	}
	if !synchronized {
		observation.Status, observation.Reason = StatusFalse, "unsynchronized"
		return observation
	}
	if absDuration(offset) > MaximumHealthyClockOffset {
		observation.Status, observation.Reason = StatusFalse, "offset_exceeded"
		return observation
	}
	observation.Status, observation.Reason = StatusTrue, "synchronized"
	return observation
}

func EvaluateSpool(measurementAvailable bool, usedBytes, configuredBytes int64, filesystemFreePercent float64) (Observation, error) {
	observation := Observation{Type: TypeSpoolHealthy}
	if !measurementAvailable {
		observation.Status, observation.Reason = StatusUnknown, "measurement_unavailable"
		return observation, nil
	}
	if usedBytes < 0 || configuredBytes <= 0 || math.IsNaN(filesystemFreePercent) || math.IsInf(filesystemFreePercent, 0) || filesystemFreePercent < 0 || filesystemFreePercent > 100 {
		return Observation{}, ErrInvalidMeasurement
	}
	ratio := float64(usedBytes) / float64(configuredBytes)
	switch {
	case ratio >= SpoolCriticalRatio || filesystemFreePercent <= DiskCriticalFreePercent:
		observation.Status, observation.Reason = StatusFalse, "capacity_critical"
	case ratio >= SpoolWarningRatio || filesystemFreePercent <= DiskWarningFreePercent:
		observation.Status, observation.Reason = StatusFalse, "capacity_warning"
	default:
		observation.Status, observation.Reason = StatusTrue, "ready"
	}
	return observation, nil
}

func EvaluateDatabase(measurementAvailable, readOK, writeOK, integrityOK bool) Observation {
	observation := Observation{Type: TypeLocalDatabaseHealthy}
	if !measurementAvailable {
		observation.Status, observation.Reason = StatusUnknown, "not_observed"
		return observation
	}
	switch {
	case !integrityOK:
		observation.Status, observation.Reason = StatusFalse, "integrity_failed"
	case !writeOK:
		observation.Status, observation.Reason = StatusFalse, "write_failed"
	case !readOK:
		observation.Status, observation.Reason = StatusFalse, "read_failed"
	default:
		observation.Status, observation.Reason = StatusTrue, "ready"
	}
	return observation
}

func EvaluateProbe(conditionType ConditionType, available, timedOut bool) (Observation, error) {
	if conditionType != TypeContainerRuntimeReachable && conditionType != TypePrivilegedHelperReachable {
		return Observation{}, ErrInvalidMeasurement
	}
	observation := Observation{Type: conditionType}
	if available {
		observation.Status, observation.Reason = StatusTrue, "reachable"
		return observation, nil
	}
	observation.Status = StatusFalse
	if timedOut {
		if conditionType == TypePrivilegedHelperReachable {
			observation.Reason = "rpc_timeout"
		} else {
			observation.Reason = "probe_timeout"
		}
		return observation, nil
	}
	if conditionType == TypePrivilegedHelperReachable {
		observation.Reason = "rpc_unavailable"
	} else {
		observation.Reason = "probe_failed"
	}
	return observation, nil
}

func absDuration(value time.Duration) time.Duration {
	if value == time.Duration(math.MinInt64) {
		return time.Duration(math.MaxInt64)
	}
	if value < 0 {
		return -value
	}
	return value
}
