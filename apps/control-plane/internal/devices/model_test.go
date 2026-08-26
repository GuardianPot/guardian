package devices

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

const testEnvironmentID = "0198dc8c-c600-7000-8000-000000000003"

type repositoryStub struct {
	record         EnrollmentTokenRecord
	certificate    Certificate
	enrollCalls    int
	failureRecords int
	clearCalls     int
	eligibleDevice string
	eligibleSerial string
}

func (r *repositoryStub) CreateEnrollmentToken(_ context.Context, record EnrollmentTokenRecord, _ string) error {
	r.record = record
	return nil
}
func (r *repositoryStub) CreateReenrollmentToken(_ context.Context, record EnrollmentTokenRecord, _ string) (string, error) {
	r.record = record
	return "edge-one", nil
}
func (*repositoryStub) ListEnrollmentTokens(context.Context, string) ([]TokenSummary, error) {
	return nil, nil
}
func (*repositoryStub) RevokeEnrollmentToken(context.Context, string, string, string, string) error {
	return nil
}
func (r *repositoryStub) Enroll(_ context.Context, _ [sha256.Size]byte, _ time.Time, issue func(string) (Certificate, error)) (Certificate, error) {
	r.enrollCalls++
	certificate, err := issue(r.record.DeviceID)
	certificate.EnvironmentID = r.record.EnvironmentID
	r.certificate = certificate
	return certificate, err
}
func (*repositoryStub) Rotate(context.Context, string, string, time.Time, func(string) (Certificate, error)) (Certificate, error) {
	return Certificate{}, nil
}
func (*repositoryStub) SetDeviceState(context.Context, string, string, DeviceState, string) error {
	return nil
}
func (r *repositoryStub) CertificateEligible(_ context.Context, deviceID, serial string) error {
	r.eligibleDevice, r.eligibleSerial = deviceID, serial
	return nil
}
func (*repositoryStub) AllowEnrollmentAttempt(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error {
	return nil
}
func (r *repositoryStub) RecordEnrollmentFailure(context.Context, [sha256.Size]byte, [sha256.Size]byte, time.Time) error {
	r.failureRecords++
	return nil
}
func (r *repositoryStub) ClearEnrollmentThrottle(context.Context, [sha256.Size]byte, [sha256.Size]byte) error {
	r.clearCalls++
	return nil
}

func TestServiceCreatesOneTimeTokenAndEnrollsBoundCSR(t *testing.T) {
	now := time.Date(2026, 8, 24, 20, 0, 0, 0, time.UTC)
	repository := new(repositoryStub)
	service := newTestService(t, repository, now)
	token, err := service.CreateEnrollmentToken(context.Background(), testEnvironmentID, "edge-one", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(token.Token)
	if err != nil || len(raw) != enrollmentTokenBytes {
		t.Fatalf("token decode length = %d, %v", len(raw), err)
	}
	wantHash := sha256.Sum256(raw)
	clear(raw)
	if repository.record.TokenHash != wantHash || repository.record.DeviceID != token.DeviceID ||
		repository.record.ExpiresAt.Sub(now) != EnrollmentTokenLifetime || token.Token == "" {
		t.Fatalf("stored token metadata = %+v, returned = %+v", repository.record, token)
	}
	csr := testCSR(t)
	result, err := service.Enroll(context.Background(), "192.0.2.10", token.Token, csr)
	if err != nil {
		t.Fatal(err)
	}
	if repository.enrollCalls != 1 || repository.clearCalls != 1 || repository.failureRecords != 0 ||
		result.Certificate.DeviceID != token.DeviceID || len(result.CAPEM) == 0 {
		t.Fatalf("enrollment result = %+v repository = %+v", result, repository)
	}
	certificate, err := devicepki.ParseCertificate(result.Certificate.PEM)
	if err != nil {
		t.Fatal(err)
	}
	deviceID, serial, err := service.VerifyCertificate(context.Background(), certificate)
	if err != nil || deviceID != token.DeviceID || serial != result.Certificate.Serial ||
		repository.eligibleDevice != deviceID || repository.eligibleSerial != serial {
		t.Fatalf("verify = %q %q %v repository = %+v", deviceID, serial, err, repository)
	}
}

func TestServiceInvalidTokenIsThrottledBeforeEnrollment(t *testing.T) {
	repository := new(repositoryStub)
	service := newTestService(t, repository, time.Now().UTC())
	_, err := service.Enroll(context.Background(), "192.0.2.11", "not-a-token", testCSR(t))
	if !errors.Is(err, ErrTokenInvalid) || repository.failureRecords != 1 || repository.enrollCalls != 0 {
		t.Fatalf("invalid enrollment = %v repository = %+v", err, repository)
	}
}

func TestServiceCreatesReenrollmentTokenForStableDevice(t *testing.T) {
	now := time.Date(2026, 8, 24, 20, 0, 0, 0, time.UTC)
	repository := new(repositoryStub)
	service := newTestService(t, repository, now)
	const deviceID = "0198dc8c-c600-7000-8000-000000000004"
	token, err := service.CreateReenrollmentToken(context.Background(), testEnvironmentID, deviceID, "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if token.DeviceID != deviceID || token.DeviceName != "edge-one" || repository.record.DeviceID != deviceID ||
		repository.record.EnvironmentID != testEnvironmentID || repository.record.ExpiresAt.Sub(now) != EnrollmentTokenLifetime {
		t.Fatalf("re-enrollment token = %+v record = %+v", token, repository.record)
	}
}

func newTestService(t *testing.T, repository Repository, now time.Time) *Service {
	t.Helper()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x62}, secretstore.MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	material, err := devicepki.GenerateMaterial(secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := devicepki.LoadProductAuthority(material, secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository, authority)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	return service
}

func testCSR(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}
