package health

import (
	"context"
	"errors"
)

var (
	ErrReportPending           = errors.New("health report acknowledgement is pending")
	ErrAcknowledgementMismatch = errors.New("health acknowledgement does not match pending report")
	ErrSequenceExhausted       = errors.New("health report sequence is exhausted")
)

type DurableState struct {
	NextSequence   uint64
	Conditions     []Condition
	PendingReport  *Report
	PendingPayload []byte
}

type StateStore interface {
	LoadHealthState(context.Context) (DurableState, error)
	PersistHealthReport(context.Context, Report, []byte) error
	AcknowledgeHealthReport(context.Context, string, uint64) error
}
