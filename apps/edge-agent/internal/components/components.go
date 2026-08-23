// Package components defines the stable Edge module interfaces and the honest
// production-skeleton implementations used before later work packages fill in
// network, reconciliation, and privileged behavior.
package components

import (
	"context"
	"errors"
	"sync"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

var ErrNotImplemented = errors.New("edge component is not implemented in P1-W7")

// Enrollment exposes only validated non-secret identity metadata.
type Enrollment interface {
	lifecycle.Component
	Identity() identity.Metadata
}

// Channel is the future mTLS device-channel boundary.
type Channel interface {
	lifecycle.Component
	ConnectionState() string
}

// Reconciler is the future desired-state convergence boundary.
type Reconciler interface {
	lifecycle.Component
	Trigger(context.Context) error
}

// HealthReporter exposes the bounded local health snapshot.
type HealthReporter interface {
	lifecycle.Component
	Snapshot(context.Context) (storage.Snapshot, error)
}

// TelemetrySpool exposes durable queue statistics without payload content.
type TelemetrySpool interface {
	lifecycle.Component
	Stats(context.Context) (storage.Stats, error)
}

// PrivilegedHelperClient is the narrow future typed-UDS client boundary. This
// package never opens a runtime socket or executes a shell command.
type PrivilegedHelperClient interface {
	lifecycle.Component
	Available() bool
}

// Graph groups explicit interfaces and produces the approved startup order.
type Graph struct {
	Enrollment             Enrollment
	TelemetrySpool         TelemetrySpool
	Channel                Channel
	Reconciler             Reconciler
	PrivilegedHelperClient PrivilegedHelperClient
	HealthReporter         HealthReporter
}

// Ordered returns enrollment, local durability, network/reconciliation,
// privileged-client, and health boundaries in dependency order.
func (g Graph) Ordered() []lifecycle.Component {
	return []lifecycle.Component{
		g.Enrollment,
		g.TelemetrySpool,
		g.Channel,
		g.Reconciler,
		g.PrivilegedHelperClient,
		g.HealthReporter,
	}
}

// NewFoundation creates passive, truthful P1-W7 boundaries. Future packages
// replace them through these interfaces without widening daemon privileges.
func NewFoundation(store *storage.Store, metadata identity.Metadata, enrollmentStatus string) Graph {
	enrollmentHealth := "healthy"
	enrollmentReason := "identity-valid"
	if enrollmentStatus == "revoked" {
		enrollmentHealth = "degraded"
		enrollmentReason = "identity-revoked"
	}
	return Graph{
		Enrollment: &enrollmentComponent{
			baseComponent: baseComponent{name: "enrollment", status: enrollmentHealth, reason: enrollmentReason, store: store},
			metadata:      metadata,
		},
		TelemetrySpool: &spoolComponent{
			baseComponent: baseComponent{name: "telemetry-spool", status: "healthy", reason: "ready", store: store},
			store:         store,
		},
		Channel: &channelComponent{
			baseComponent: baseComponent{name: "device-channel", status: "degraded", reason: "not-implemented", store: store},
		},
		Reconciler: &reconcilerComponent{
			baseComponent: baseComponent{name: "reconciler", status: "degraded", reason: "not-implemented", store: store},
		},
		PrivilegedHelperClient: &helperComponent{
			baseComponent: baseComponent{name: "privileged-helper", status: "degraded", reason: "not-implemented", store: store},
		},
		HealthReporter: &healthComponent{
			baseComponent: baseComponent{name: "health-reporter", status: "healthy", reason: "ready", store: store},
			store:         store,
		},
	}
}

type baseComponent struct {
	name    string
	status  string
	reason  string
	store   *storage.Store
	mu      sync.RWMutex
	started bool
}

func (c *baseComponent) Name() string { return c.name }

func (c *baseComponent) Start(ctx context.Context) error {
	if err := c.store.SetHealth(ctx, storage.HealthCondition{Name: c.name, Status: c.status, ReasonCode: c.reason}); err != nil {
		return err
	}
	c.mu.Lock()
	c.started = true
	c.mu.Unlock()
	return nil
}

func (c *baseComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	wasStarted := c.started
	c.started = false
	c.mu.Unlock()
	if !wasStarted {
		return nil
	}
	return c.store.SetHealth(ctx, storage.HealthCondition{Name: c.name, Status: "stopped", ReasonCode: "shutdown"})
}

type enrollmentComponent struct {
	baseComponent
	metadata identity.Metadata
}

func (c *enrollmentComponent) Identity() identity.Metadata { return c.metadata }

type channelComponent struct{ baseComponent }

func (*channelComponent) ConnectionState() string { return "not-implemented" }

type reconcilerComponent struct{ baseComponent }

func (*reconcilerComponent) Trigger(context.Context) error { return ErrNotImplemented }

type spoolComponent struct {
	baseComponent
	store *storage.Store
}

func (c *spoolComponent) Stats(ctx context.Context) (storage.Stats, error) {
	return c.store.Stats(ctx)
}

type helperComponent struct{ baseComponent }

func (*helperComponent) Available() bool { return false }

type healthComponent struct {
	baseComponent
	store *storage.Store
}

func (c *healthComponent) Snapshot(ctx context.Context) (storage.Snapshot, error) {
	return c.store.Snapshot(ctx)
}

var (
	_ Enrollment             = (*enrollmentComponent)(nil)
	_ Channel                = (*channelComponent)(nil)
	_ Reconciler             = (*reconcilerComponent)(nil)
	_ HealthReporter         = (*healthComponent)(nil)
	_ TelemetrySpool         = (*spoolComponent)(nil)
	_ PrivilegedHelperClient = (*helperComponent)(nil)
)
