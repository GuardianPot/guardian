package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type testComponent struct {
	name       string
	events     *[]string
	startError error
	stopError  error
}

func (c *testComponent) Name() string { return c.name }
func (c *testComponent) Start(context.Context) error {
	*c.events = append(*c.events, "start:"+c.name)
	return c.startError
}
func (c *testComponent) Stop(context.Context) error {
	*c.events = append(*c.events, "stop:"+c.name)
	return c.stopError
}

func TestManagerStartsAndStopsInExplicitOrder(t *testing.T) {
	var events []string
	manager, err := New(
		&testComponent{name: "first", events: &events},
		&testComponent{name: "second", events: &events},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"start:first", "start:second", "stop:second", "stop:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestManagerRollsBackStartedPrefixAfterFailure(t *testing.T) {
	var events []string
	startErr := errors.New("start failed")
	manager, err := New(
		&testComponent{name: "first", events: &events},
		&testComponent{name: "second", events: &events, startError: startErr},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); !errors.Is(err, startErr) {
		t.Fatalf("Start() error = %v, want start failure", err)
	}
	want := []string{"start:first", "start:second", "stop:first"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestManagerRejectsDuplicateNames(t *testing.T) {
	var events []string
	if _, err := New(
		&testComponent{name: "duplicate", events: &events},
		&testComponent{name: "duplicate", events: &events},
	); err == nil {
		t.Fatal("New() unexpectedly accepted duplicate component names")
	}
}
