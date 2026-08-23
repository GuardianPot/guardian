// Package lifecycle starts components in order and stops them in reverse order.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
)

// Component is an explicitly ordered in-process module boundary.
type Component interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

// Manager owns a fixed component graph.
type Manager struct {
	components []Component
	started    []Component
}

// New validates that component names are non-empty and unique.
func New(components ...Component) (*Manager, error) {
	seen := make(map[string]struct{}, len(components))
	for _, component := range components {
		if component == nil || component.Name() == "" {
			return nil, errors.New("component name is required")
		}
		if _, exists := seen[component.Name()]; exists {
			return nil, fmt.Errorf("duplicate component %q", component.Name())
		}
		seen[component.Name()] = struct{}{}
	}
	return &Manager{components: append([]Component(nil), components...)}, nil
}

// Start starts each component in order. A failed start rolls back the already
// started prefix in reverse order.
func (m *Manager) Start(ctx context.Context) error {
	for _, component := range m.components {
		if err := component.Start(ctx); err != nil {
			stopErr := m.Stop(context.Background())
			return errors.Join(fmt.Errorf("start component %s: %w", component.Name(), err), stopErr)
		}
		m.started = append(m.started, component)
	}
	return nil
}

// Stop stops only successfully started components, in reverse order.
func (m *Manager) Stop(ctx context.Context) error {
	var joined error
	for index := len(m.started) - 1; index >= 0; index-- {
		component := m.started[index]
		if err := component.Stop(ctx); err != nil {
			joined = errors.Join(joined, fmt.Errorf("stop component %s: %w", component.Name(), err))
		}
	}
	m.started = nil
	return joined
}
