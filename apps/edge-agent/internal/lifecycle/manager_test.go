package lifecycle

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeComponent struct {
	name     string
	events   *[]string
	startErr error
	stopErr  error
}

func (c *fakeComponent) Name() string { return c.name }

func (c *fakeComponent) Start(context.Context) error {
	*c.events = append(*c.events, "start:"+c.name)
	return c.startErr
}

func (c *fakeComponent) Stop(context.Context) error {
	*c.events = append(*c.events, "stop:"+c.name)
	return c.stopErr
}

func TestManagerStartsInOrderAndStopsInReverse(t *testing.T) {
	var events []string
	manager, err := New(
		&fakeComponent{name: "enrollment", events: &events},
		&fakeComponent{name: "channel", events: &events},
		&fakeComponent{name: "health", events: &events},
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
	want := []string{"start:enrollment", "start:channel", "start:health", "stop:health", "stop:channel", "stop:enrollment"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestManagerRollsBackStartedComponentsOnFailure(t *testing.T) {
	var events []string
	startFailure := errors.New("channel unavailable")
	manager, err := New(
		&fakeComponent{name: "enrollment", events: &events},
		&fakeComponent{name: "channel", events: &events, startErr: startFailure},
		&fakeComponent{name: "health", events: &events},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); !errors.Is(err, startFailure) {
		t.Fatalf("Start() error = %v", err)
	}
	want := []string{"start:enrollment", "start:channel", "stop:enrollment"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestManagerRejectsInvalidGraph(t *testing.T) {
	var events []string
	if _, err := New((*fakeComponent)(nil)); err == nil {
		t.Fatal("nil component unexpectedly accepted")
	}
	if _, err := New(&fakeComponent{name: "same", events: &events}, &fakeComponent{name: "same", events: &events}); err == nil {
		t.Fatal("duplicate component unexpectedly accepted")
	}
}
