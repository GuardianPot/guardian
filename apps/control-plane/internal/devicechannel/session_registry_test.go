package devicechannel

import (
	"context"
	"testing"
)

func TestSessionRegistryRemovesAndDisconnectsOnlyCurrentSession(t *testing.T) {
	registry := sessionRegistry{active: make(map[string]*activeSession)}
	identity := DeviceIdentity{DeviceID: "0198dc8c-c600-7000-8000-000000000004", CertificateSerial: "01"}
	first := newActiveSession(context.Background(), identity)
	second := newActiveSession(context.Background(), identity)
	registry.activate(first)
	registry.activate(second)
	select {
	case <-first.ctx.Done():
	default:
		t.Fatal("replacement did not cancel the older session")
	}
	if registry.remove(first) {
		t.Fatal("older replacement session was treated as current")
	}
	if registry.get(identity.DeviceID) != second {
		t.Fatal("removing older session removed the current session")
	}
	if !registry.remove(second) || registry.get(identity.DeviceID) != nil {
		t.Fatal("current session was not removed exactly once")
	}
}
