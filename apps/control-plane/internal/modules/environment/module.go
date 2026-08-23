// Package environment is the Control Plane environment module boundary.
package environment

import "context"

type Module struct{}

func New() *Module                          { return &Module{} }
func (*Module) Name() string                { return "environment" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
