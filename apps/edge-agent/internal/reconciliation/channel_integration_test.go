//go:build integration

package reconciliation

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel"
	devicev1 "github.com/GuardianPot/guardian/apps/edge-agent/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/identity"
	"github.com/GuardianPot/guardian/apps/edge-agent/internal/storage"
	"google.golang.org/protobuf/proto"
)

type channelFixture struct {
	Endpoint       string `json:"endpoint"`
	DeviceID       string `json:"device_id"`
	Certificate    string `json:"certificate"`
	PrivateKey     string `json:"private_key"`
	ServerCA       string `json:"server_ca"`
	ExpectedZoneID string `json:"expected_zone_id"`
}

func TestReconciliationEdgeFixture(t *testing.T) {
	fixtureDirectory := os.Getenv("GUARDIAN_RECON_FIXTURE_DIR")
	if fixtureDirectory == "" {
		t.Fatal("GUARDIAN_RECON_FIXTURE_DIR is required")
	}
	fixturePayload, err := os.ReadFile(filepath.Join(fixtureDirectory, "fixture.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture channelFixture
	if err := json.Unmarshal(fixturePayload, &fixture); err != nil {
		t.Fatal(err)
	}
	credentials := loadFixtureCredentials(t, fixture)
	if credentials.Metadata.DeviceID != fixture.DeviceID {
		t.Fatalf("credential device = %s, want %s", credentials.Metadata.DeviceID, fixture.DeviceID)
	}
	rootPEM, err := os.ReadFile(fixture.ServerCA)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(rootPEM) {
		t.Fatal("server CA is invalid")
	}
	root := t.TempDir()
	store, err := storage.Open(context.Background(), storage.Options{
		DatabasePath: filepath.Join(root, "edge.db"), SpoolDirectory: filepath.Join(root, "spool"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var client *devicechannel.Client
	reconciler, err := New(store, fixture.DeviceID, PublisherFunc(func(observed *devicev1.ObservedState) error {
		return client.EnqueueObserved(observed)
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	client, err = devicechannel.NewClient(devicechannel.Config{
		Endpoint: fixture.Endpoint, AgentVersion: "guardian-edge/p1-w6-integration",
		Credentials: credentials, RootCAs: roots, Logger: logger,
		DesiredHandler: reconciler, ObservedAcknowledgementHandler: reconciler,
		StateRecorder: devicechannel.StateRecorderFunc(func(ctx context.Context, state, reason string) error {
			status := "degraded"
			if state == "connected" {
				status = "healthy"
			}
			return store.SetHealth(ctx, storage.HealthCondition{Name: "device-channel", Status: status, ReasonCode: reason})
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reconciler.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopContext, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = reconciler.Stop(stopContext)
		_ = client.Stop(stopContext)
	}()

	for ctx.Err() == nil {
		record, stateErr := store.ReconciliationState(ctx)
		if stateErr == nil && record.ConditionStatus == "converged" && record.LastGoodRevision == 1 && !record.ObservedPending {
			var applied devicev1.DesiredStateSnapshot
			if err := proto.Unmarshal(record.LastGoodPayload, &applied); err != nil {
				t.Fatal(err)
			}
			if len(applied.Zones) != 1 || applied.Zones[0].ZoneId != fixture.ExpectedZoneID {
				t.Fatalf("applied zones = %+v", applied.Zones)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("Edge did not converge and persist the real channel snapshot: %v", ctx.Err())
}

func loadFixtureCredentials(t *testing.T, fixture channelFixture) identity.Credentials {
	t.Helper()
	credentials, err := identity.LoadCredentials(fixture.Certificate, fixture.PrivateKey, time.Now().UTC())
	if err == nil {
		return credentials
	}
	// Windows does not expose useful Unix owner/group mode bits for a file made
	// by the sibling Control Plane process. Production and Linux CI still take
	// the protected-file loader path; this local fixture-only fallback preserves
	// certificate/key and device-SAN validation for the real mTLS proof.
	if runtime.GOOS != "windows" || !errors.Is(err, identity.ErrPermissions) {
		t.Fatal(err)
	}
	pair, err := tls.LoadX509KeyPair(fixture.Certificate, fixture.PrivateKey)
	if err != nil || len(pair.Certificate) == 0 {
		t.Fatalf("load fixture keypair: %v", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	wantURI := "urn:guardian:device:" + fixture.DeviceID
	if len(leaf.URIs) != 1 || leaf.URIs[0].String() != wantURI {
		t.Fatalf("fixture device URI SAN = %v, want %s", leaf.URIs, wantURI)
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	pair.Leaf = leaf
	return identity.Credentials{
		Certificate: pair,
		Leaf:        leaf,
		Metadata: identity.Metadata{
			DeviceID: fixture.DeviceID, CertificateSerial: leaf.SerialNumber.Text(16),
			CertificateSHA256: hex.EncodeToString(fingerprint[:]),
			NotBefore:         leaf.NotBefore.UTC(), NotAfter: leaf.NotAfter.UTC(),
		},
	}
}
