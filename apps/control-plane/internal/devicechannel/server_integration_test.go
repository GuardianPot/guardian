//go:build integration

package devicechannel

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	devicev1 "github.com/GuardianPot/guardian/apps/control-plane/internal/devicechannel/gen/guardian/device/v1"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

func TestDurableP1W4VerifierRejectsRevokedActiveChannel(t *testing.T) {
	ctx := context.Background()
	databaseURL := createChannelTestDatabase(t)
	if _, err := storage.Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(ctx, databaseURL, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x5a}, secretstore.MasterKeyBytes))
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
	created, err := environmentService.CreateEnvironment(ctx, "Channel integration", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := deviceService.CreateEnrollmentToken(ctx, created.EnvironmentID, "channel-edge", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	clientKey, csr := channelCSR(t)
	enrollment, err := deviceService.Enroll(ctx, "192.0.2.50", token.Token, csr)
	if err != nil {
		t.Fatal(err)
	}
	clientKeyDER, _ := x509.MarshalPKCS8PrivateKey(clientKey)
	clientCertificate, err := tls.X509KeyPair(
		append(enrollment.Certificate.PEM, enrollment.CAPEM...),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: clientKeyDER}),
	)
	if err != nil {
		t.Fatal(err)
	}

	serverIdentity := newChannelServerIdentity(t)
	server, err := NewServer(Config{
		Address: "127.0.0.1:0", TLSCertificateFile: serverIdentity.certificateFile,
		TLSPrivateKeyFile: serverIdentity.privateKeyFile, DeviceCAPEM: authority.CertificatePEM(),
		Verifier: deviceService, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	server.recheckInterval = 10 * time.Millisecond
	server.staleAfter = time.Second
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()

	connection, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13, ServerName: "localhost", RootCAs: serverIdentity.roots,
		Certificates: []tls.Certificate{clientCertificate},
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	stream, err := devicev1.NewDeviceChannelServiceClient(connection).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(helloFrame(ProtocolMajor, ProtocolMinor)); err != nil {
		t.Fatal(err)
	}
	selection, err := stream.Recv()
	if err != nil || selection.GetProtocolSelection() == nil || !selection.GetProtocolSelection().Accepted {
		t.Fatalf("durable verifier negotiation = %+v error=%v", selection, err)
	}
	if err := deviceService.SetDeviceState(ctx, created.EnvironmentID, token.DeviceID, devices.DeviceRevoked, "owner-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("revoked active durable channel error = %v", err)
	}
}

type channelServerIdentity struct {
	certificateFile string
	privateKeyFile  string
	roots           *x509.CertPool
}

func newChannelServerIdentity(t *testing.T) channelServerIdentity {
	t.Helper()
	now := time.Now().UTC()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(90), Subject: pkix.Name{CommonName: "Channel server test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	ca, _ := x509.ParseCertificate(caDER)
	serverKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(91), DNSNames: []string{"localhost"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	serverDER, _ := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	serverKeyDER, _ := x509.MarshalPKCS8PrivateKey(serverKey)
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
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return channelServerIdentity{certificateFile: certificateFile, privateKeyFile: privateKeyFile, roots: roots}
}

func channelCSR(t *testing.T) (*ecdsa.PrivateKey, []byte) {
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

func createChannelTestDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("GUARDIAN_TEST_DATABASE_URL")
	if base == "" {
		t.Fatal("GUARDIAN_TEST_DATABASE_URL is required for integration tests")
	}
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("guardian_channel_%d", time.Now().UnixNano())
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
			t.Errorf("drop database: %v", err)
		}
	})
	return testURL.String()
}
