package components

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/lifecycle"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
)

func TestFoundationHasExplicitOrderAndTruthfulSkeletonState(t *testing.T) {
	root := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{
		DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	metadata := identity.Metadata{CertificateSHA256: "fingerprint", NotAfter: time.Now().Add(time.Hour)}
	graph := NewFoundation(store, metadata, "enrolled")
	ordered := graph.Ordered()
	names := make([]string, 0, len(ordered))
	for _, component := range ordered {
		names = append(names, component.Name())
	}
	want := []string{"enrollment", "telemetry-spool", "device-channel", "reconciler", "privileged-helper", "health-reporter"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("component order = %v, want %v", names, want)
	}
	manager, err := lifecycle.New(ordered...)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if graph.Channel.ConnectionState() != "not-implemented" || graph.PrivilegedHelperClient.Available() {
		t.Fatal("skeleton component reported capability it does not have")
	}
	if err := graph.Reconciler.Trigger(context.Background()); !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("Trigger() error = %v", err)
	}
	snapshot, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Health) != len(want) {
		t.Fatalf("health conditions = %+v", snapshot.Health)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	revoked := NewFoundation(store, metadata, "revoked")
	revokedManager, err := lifecycle.New(revoked.Ordered()...)
	if err != nil {
		t.Fatal(err)
	}
	if err := revokedManager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	revokedSnapshot, err := store.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	foundRevoked := false
	for _, condition := range revokedSnapshot.Health {
		if condition.Name == "enrollment" && condition.Status == "degraded" && condition.ReasonCode == "identity-revoked" {
			foundRevoked = true
		}
	}
	if !foundRevoked {
		t.Fatalf("revoked identity was not reported honestly: %+v", revokedSnapshot.Health)
	}
	if err := revokedManager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
