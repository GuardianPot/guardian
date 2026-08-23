// Package deception is the placeholder deception-configuration module boundary.
package deception

import "context"

type Module struct{}

func New() *Module                          { return &Module{} }
func (*Module) Name() string                { return "deception" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
