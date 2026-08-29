package reconciliation

import "context"

type Module struct{}

func New() *Module                          { return &Module{} }
func (*Module) Name() string                { return "reconciliation" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
