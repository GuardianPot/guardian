package privileged

import (
	"context"
	"log/slog"
)

// AuditEvent is deliberately metadata-only. Request payloads, addresses, and
// policy objects are represented only by a deterministic digest.
type AuditEvent struct {
	Event              string
	Method             string
	RequestID          string
	RequestFingerprint string
	Outcome            string
	ReasonCode         string
	PeerPID            int32
	PeerUID            uint32
	PeerGID            uint32
}

// AuditRecorder receives security-relevant helper connection and RPC events.
type AuditRecorder interface {
	Record(context.Context, AuditEvent)
}

type slogAuditRecorder struct {
	logger *slog.Logger
}

// NewSlogAuditRecorder writes redacted structured events to the service log.
func NewSlogAuditRecorder(logger *slog.Logger) AuditRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	return &slogAuditRecorder{logger: logger}
}

func (r *slogAuditRecorder) Record(ctx context.Context, event AuditEvent) {
	r.logger.InfoContext(ctx, "privileged helper audit",
		"event", event.Event,
		"method", event.Method,
		"request_id", event.RequestID,
		"request_fingerprint", event.RequestFingerprint,
		"outcome", event.Outcome,
		"reason_code", event.ReasonCode,
		"peer_pid", event.PeerPID,
		"peer_uid", event.PeerUID,
		"peer_gid", event.PeerGID,
	)
}
