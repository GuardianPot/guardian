// Package jobs is the durable jobs/outbox module boundary.
package jobs

import "context"

type Module struct{}

func New() *Module                          { return &Module{} }
func (*Module) Name() string                { return "jobs" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
