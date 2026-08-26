package enrollment

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
)

const (
	rotationWindow      = 10 * 24 * time.Hour
	minimumRetry        = time.Minute
	maximumRetry        = 15 * time.Minute
	minimumExpiryMargin = time.Hour
)

// RotationManager automatically rotates within the approved final ten-day
// window. Its deterministic schedule is derived from non-secret certificate
// metadata, so service restart preserves the same rotation point.
type RotationManager struct {
	endpoint string
	certPath string
	keyPath  string
	client   *Client
	logger   *slog.Logger
	now      func() time.Time

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewRotationManager(endpoint, certPath, keyPath string, client *Client, logger *slog.Logger) *RotationManager {
	if client == nil {
		client = &Client{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &RotationManager{
		endpoint: endpoint, certPath: certPath, keyPath: keyPath,
		client: client, logger: logger, now: time.Now,
	}
}

func (*RotationManager) Name() string { return "certificate-rotation" }

func (m *RotationManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return errors.New("certificate rotation manager already started")
	}
	loopCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})
	go m.loop(loopCtx)
	return nil
}

func (m *RotationManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	m.cancel = nil
	m.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *RotationManager) loop(ctx context.Context) {
	defer close(m.done)
	retry := minimumRetry
	for {
		metadata, err := identity.Load(m.certPath, m.keyPath, m.now().UTC())
		if err != nil {
			m.logger.WarnContext(ctx, "certificate rotation identity unavailable")
			if !waitFor(ctx, retry) {
				return
			}
			retry = nextRetry(retry)
			continue
		}
		due := rotationTime(metadata)
		if delay := due.Sub(m.now().UTC()); delay > 0 {
			if !waitFor(ctx, delay) {
				return
			}
		}
		if _, err := m.client.Rotate(ctx, m.endpoint, m.certPath, m.keyPath); err != nil {
			m.logger.WarnContext(ctx, "device certificate rotation failed")
			if !waitFor(ctx, retry) {
				return
			}
			retry = nextRetry(retry)
			continue
		}
		retry = minimumRetry
	}
}

func rotationTime(metadata identity.Metadata) time.Time {
	windowStart := metadata.NotAfter.Add(-rotationWindow)
	available := rotationWindow - minimumExpiryMargin
	digest := sha256.Sum256([]byte(metadata.CertificateSHA256))
	offset := time.Duration(binary.BigEndian.Uint64(digest[:8]) % uint64(available))
	return windowStart.Add(offset).UTC()
}

func waitFor(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func nextRetry(current time.Duration) time.Duration {
	if current >= maximumRetry/2 {
		return maximumRetry
	}
	return current * 2
}
