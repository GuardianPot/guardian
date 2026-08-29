//go:build integration

package reconciliation_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	controlreconciliation "github.com/GuardianPot/guardian/apps/control-plane/internal/reconciliation"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage"
	"github.com/jackc/pgx/v5"
)

type reconciliationFixture struct {
	Endpoint       string `json:"endpoint"`
	DeviceID       string `json:"device_id"`
	Certificate    string `json:"certificate"`
	PrivateKey     string `json:"private_key"`
	ServerCA       string `json:"server_ca"`
	ExpectedZoneID string `json:"expected_zone_id"`
}

func TestReconciliationControlFixture(t *testing.T) {
	fixtureDirectory := os.Getenv("GUARDIAN_RECON_FIXTURE_DIR")
	if fixtureDirectory == "" {
		t.Fatal("GUARDIAN_RECON_FIXTURE_DIR is required")
	}
	ctx := context.Background()
	databaseURL := createReconciliationDatabase(t)
	if _, err := storage.Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, databaseURL, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x6b}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	material, err := devicepki.GenerateMaterial(secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeDeviceCA(ctx, material); err != nil {
		t.Fatal(err)
	}
	authority, err := devicepki.LoadProductAuthority(material, secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	deviceService, err := devices.NewService(store, authority)
	if err != nil {
		t.Fatal(err)
	}
	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	created, err := environmentService.CreateEnvironment(ctx, "Reconciliation channel", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	zone, err := environmentService.CreateZone(ctx, created.EnvironmentID, "Primary", "10.44.0.0/24", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := deviceService.CreateEnrollmentToken(ctx, created.EnvironmentID, "reconciliation-edge", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	clientKey, csr := reconciliationCSR(t)
	enrollment, err := deviceService.Enroll(ctx, "192.0.2.60", token.Token, csr)
	if err != nil {
		t.Fatal(err)
	}
	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}

	reconciliationService, err := controlreconciliation.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	reconciliationHandler, err := controlreconciliation.NewChannelHandler(reconciliationService)
	if err != nil {
		t.Fatal(err)
	}
	serverIdentity := newReconciliationServerIdentity(t)
	server, err := devicechannel.NewServer(devicechannel.Config{
		Address: "127.0.0.1:0", TLSCertificateFile: serverIdentity.certificateFile,
		TLSPrivateKeyFile: serverIdentity.privateKeyFile, DeviceCAPEM: authority.CertificatePEM(),
		Verifier: deviceService, Reconciliation: reconciliationHandler,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	_, port, err := net.SplitHostPort(server.Address())
	if err != nil {
		t.Fatal(err)
	}
	certificatePath := filepath.Join(fixtureDirectory, "device.crt")
	privateKeyPath := filepath.Join(fixtureDirectory, "device.key")
	serverCAPath := filepath.Join(fixtureDirectory, "server-ca.crt")
	if err := os.WriteFile(certificatePath, enrollment.Certificate.PEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(serverCAPath, serverIdentity.caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	fixturePayload, err := json.Marshal(reconciliationFixture{
		Endpoint: "localhost:" + port, DeviceID: token.DeviceID,
		Certificate: certificatePath, PrivateKey: privateKeyPath,
		ServerCA: serverCAPath, ExpectedZoneID: zone.ZoneID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixtureDirectory, "fixture.json"), fixturePayload, 0o600); err != nil {
		t.Fatal(err)
	}
	queryConnection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer queryConnection.Close(ctx)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		var observedRevision, lastGoodRevision int64
		var acknowledged bool
		err := queryConnection.QueryRow(ctx, `
SELECT o.condition_status, o.observed_revision, o.last_good_revision,
       d.acknowledged_at IS NOT NULL
FROM guardian_reconciliation.observed_state o
JOIN guardian_reconciliation.desired_state_revisions d
  ON d.device_id = o.device_id AND d.revision = o.desired_revision
WHERE o.device_id = $1`, token.DeviceID).Scan(&status, &observedRevision, &lastGoodRevision, &acknowledged)
		if err == nil && status == "converged" && observedRevision == 1 && lastGoodRevision == 1 && acknowledged {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("real device channel did not persist converged desired/observed state")
}

type reconciliationServerIdentity struct {
	certificateFile string
	privateKeyFile  string
	caPEM           []byte
}

func newReconciliationServerIdentity(t *testing.T) reconciliationServerIdentity {
	t.Helper()
	now := time.Now().UTC()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(190), Subject: pkix.Name{CommonName: "Reconciliation server test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true,
		BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(191), DNSNames: []string{"localhost"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	serverKeyDER, err := x509.MarshalPKCS8PrivateKey(serverKey)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	certificateFile := filepath.Join(root, "server.crt")
	privateKeyFile := filepath.Join(root, "server.key")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if err := os.WriteFile(certificateFile, append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: serverDER}), caPEM...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privateKeyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKeyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return reconciliationServerIdentity{certificateFile: certificateFile, privateKeyFile: privateKeyFile, caPEM: caPEM}
}

func reconciliationCSR(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	request, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		t.Fatal(err)
	}
	return key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: request})
}

func createReconciliationDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GUARDIAN_TEST_DATABASE_URL")
	if base == "" {
		t.Fatal("GUARDIAN_TEST_DATABASE_URL is required")
	}
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("guardian_reconciliation_%d", time.Now().UnixNano())
	admin, err := pgx.Connect(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+pgx.Identifier{databaseName}.Sanitize()); err != nil {
		admin.Close(context.Background())
		t.Fatal(err)
	}
	admin.Close(context.Background())
	testURL := *parsed
	testURL.Path = "/" + databaseName
	t.Cleanup(func() {
		admin, err := pgx.Connect(context.Background(), base)
		if err != nil {
			t.Errorf("reconnect for database cleanup: %v", err)
			return
		}
		defer admin.Close(context.Background())
		_, err = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{databaseName}.Sanitize()+" WITH (FORCE)")
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("drop reconciliation database: %v", err)
		}
	})
	return testURL.String()
}
