// Package api is the public REST API module boundary.
package api

import "context"

type Module struct{}

func NewModule() *Module                    { return &Module{} }
func (*Module) Name() string                { return "api" }
func (*Module) Start(context.Context) error { return nil }
func (*Module) Stop(context.Context) error  { return nil }
