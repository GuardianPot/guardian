// Package audit is the append-oriented audit module boundary.
package audit

import "context"

type Module struct{}

func New() *Module                          { return &Module{} }
func (*Module) Name() string                { return "audit" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
