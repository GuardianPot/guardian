//go:build integration

package storage

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
	"sync"
	"testing"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/devicepki"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/devices"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/environment"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

func TestDurableEnrollmentReplayReenrollmentRotationAndRevocation(t *testing.T) {
	ctx := context.Background()
	databaseURL := createTestDatabase(t)
	if _, err := Migrate(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	store, err := Open(ctx, databaseURL, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	secrets, err := secretstore.NewLocal(bytes.Repeat([]byte{0x7a}, secretstore.MasterKeyBytes))
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
	loaded, err := store.DeviceCAMaterial(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(loaded.PrivateKeyEnvelope, []byte("PRIVATE KEY")) || bytes.Equal(loaded.PrivateKeyEnvelope, material.CertificatePEM) {
		t.Fatal("database contains plaintext or misbound device CA private material")
	}
	authority, err := devicepki.LoadProductAuthority(loaded, secrets, now)
	if err != nil {
		t.Fatal(err)
	}
	service, err := devices.NewService(store, authority)
	if err != nil {
		t.Fatal(err)
	}
	environmentService, err := environment.NewService(store)
	if err != nil {
		t.Fatal(err)
	}
	configuredEnvironment, err := environmentService.CreateEnvironment(ctx, "Enrollment integration", environment.Mutation{ActorID: "owner-1"})
	if err != nil {
		t.Fatal(err)
	}

	environmentID := configuredEnvironment.EnvironmentID
	expiredToken, err := service.CreateEnrollmentToken(ctx, environmentID, "expired-edge", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	expiredRaw, _ := base64.RawURLEncoding.DecodeString(expiredToken.Token)
	expiredHash := sha256.Sum256(expiredRaw)
	clear(expiredRaw)
	if _, err := store.Enroll(ctx, expiredHash, expiredToken.ExpiresAt, func(string) (devices.Certificate, error) {
		return devices.Certificate{}, errors.New("issuer must not run for an expired token")
	}); !errors.Is(err, devices.ErrTokenExpired) {
		t.Fatalf("expired token error = %v", err)
	}
	revokedToken, err := service.CreateEnrollmentToken(ctx, environmentID, "revoked-token-edge", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RevokeEnrollmentToken(ctx, environmentID, revokedToken.TokenID, "owner-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Enroll(ctx, "192.0.2.9", revokedToken.Token, enrollmentIntegrationCSR(t)); !errors.Is(err, devices.ErrTokenRevoked) {
		t.Fatalf("revoked token error = %v", err)
	}

	token, err := service.CreateEnrollmentToken(ctx, environmentID, "integration-edge", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	rawToken, err := base64.RawURLEncoding.DecodeString(token.Token)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(rawToken)
	var storedHash []byte
	if err := store.pool.QueryRow(ctx, `SELECT token_hash FROM guardian_devices.enrollment_tokens WHERE token_id = $1`, token.TokenID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if len(storedHash) != 32 || bytes.Equal(storedHash, rawToken) {
		t.Fatal("enrollment token was not persisted exclusively as its SHA-256 hash")
	}

	csr := enrollmentIntegrationCSR(t)
	type outcome struct {
		result devices.EnrollmentResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	var start sync.WaitGroup
	start.Add(1)
	for index := 0; index < 2; index++ {
		go func(source string) {
			start.Wait()
			result, enrollErr := service.Enroll(ctx, source, token.Token, csr)
			outcomes <- outcome{result: result, err: enrollErr}
		}("192.0.2." + string(rune('1'+index)))
	}
	start.Done()
	var enrolled devices.EnrollmentResult
	successes, replays := 0, 0
	for range 2 {
		result := <-outcomes
		switch {
		case result.err == nil:
			successes++
			enrolled = result.result
		case errors.Is(result.err, devices.ErrTokenConsumed):
			replays++
		default:
			t.Fatalf("unexpected concurrent enrollment result: %v", result.err)
		}
	}
	if successes != 1 || replays != 1 {
		t.Fatalf("concurrent enrollment successes=%d replay_denials=%d", successes, replays)
	}
	firstCertificate, err := devicepki.ParseCertificate(enrolled.Certificate.PEM)
	if err != nil {
		t.Fatal(err)
	}
	if deviceID, serial, err := service.VerifyCertificate(ctx, firstCertificate); err != nil || deviceID != token.DeviceID || serial != enrolled.Certificate.Serial {
		t.Fatalf("first certificate verification = %q %q %v", deviceID, serial, err)
	}

	if err := service.SetDeviceState(ctx, environmentID, token.DeviceID, devices.DeviceDisabled, "owner-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.VerifyCertificate(ctx, firstCertificate); !errors.Is(err, devices.ErrDeviceDisabled) {
		t.Fatalf("disabled certificate verification error = %v", err)
	}
	reenrollment, err := service.CreateReenrollmentToken(ctx, environmentID, token.DeviceID, "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if reenrollment.DeviceID != token.DeviceID || reenrollment.Token == token.Token {
		t.Fatalf("re-enrollment identity/token = %+v", reenrollment)
	}
	if _, _, err := service.VerifyCertificate(ctx, firstCertificate); !errors.Is(err, devices.ErrCertificateRevoked) {
		t.Fatalf("retired certificate verification error = %v", err)
	}
	secondEnrollment, err := service.Enroll(ctx, "192.0.2.3", reenrollment.Token, enrollmentIntegrationCSR(t))
	if err != nil {
		t.Fatal(err)
	}
	if secondEnrollment.Certificate.DeviceID != token.DeviceID || secondEnrollment.Certificate.Serial == enrolled.Certificate.Serial {
		t.Fatalf("re-enrolled certificate = %+v", secondEnrollment.Certificate)
	}

	tooEarlyCSR := enrollmentIntegrationCSR(t)
	if _, err := store.Rotate(ctx, token.DeviceID, secondEnrollment.Certificate.Serial, now, func(stableID string) (devices.Certificate, error) {
		issued, issueErr := authority.Issue(stableID, tooEarlyCSR, now)
		if issueErr != nil {
			return devices.Certificate{}, issueErr
		}
		return integrationCertificate(issued), nil
	}); err == nil {
		t.Fatal("rotation outside the final ten-day window unexpectedly succeeded")
	}
	rotationNow := secondEnrollment.Certificate.NotAfter.Add(-5 * 24 * time.Hour)
	rotated, err := store.Rotate(ctx, token.DeviceID, secondEnrollment.Certificate.Serial, rotationNow, func(stableID string) (devices.Certificate, error) {
		issued, issueErr := authority.Issue(stableID, enrollmentIntegrationCSR(t), rotationNow)
		if issueErr != nil {
			return devices.Certificate{}, issueErr
		}
		return integrationCertificate(issued), nil
	})
	if err != nil || rotated.Serial == secondEnrollment.Certificate.Serial {
		t.Fatalf("inside-window rotation = %+v %v", rotated, err)
	}
	if err := store.CertificateEligible(ctx, token.DeviceID, secondEnrollment.Certificate.Serial); !errors.Is(err, devices.ErrCertificateRevoked) {
		t.Fatalf("rotated prior serial eligibility = %v", err)
	}

	if err := service.SetDeviceState(ctx, environmentID, token.DeviceID, devices.DeviceRevoked, "owner-1"); err != nil {
		t.Fatal(err)
	}
	var activeCertificates int
	if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_devices.certificates WHERE device_id = $1 AND state = 'active'`, token.DeviceID).Scan(&activeCertificates); err != nil {
		t.Fatal(err)
	}
	if activeCertificates != 0 {
		t.Fatalf("active certificates after revocation = %d", activeCertificates)
	}
	if err := store.CertificateEligible(ctx, token.DeviceID, rotated.Serial); !errors.Is(err, devices.ErrDeviceRevoked) {
		t.Fatalf("revoked device certificate eligibility = %v", err)
	}
	for _, action := range []string{
		"device.enrollment_token.created", "device.enrollment.succeeded",
		"device.certificate.issued", "device.certificate.rotated",
		"device.disabled", "device.revoked",
	} {
		var count int
		if err := store.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guardian_audit.records WHERE action = $1`, action).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count < 1 {
			t.Fatalf("missing durable audit action %q", action)
		}
	}
	invalidToken := "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	const throttledSource = "192.0.2.44"
	for attempt := 0; attempt < 5; attempt++ {
		if _, err := service.Enroll(ctx, throttledSource, invalidToken, enrollmentIntegrationCSR(t)); !errors.Is(err, devices.ErrTokenInvalid) {
			t.Fatalf("invalid enrollment attempt %d error = %v", attempt+1, err)
		}
	}
	if _, err := service.Enroll(ctx, throttledSource, invalidToken, enrollmentIntegrationCSR(t)); !errors.Is(err, devices.ErrEnrollmentRateLimited) {
		t.Fatalf("rate-limited enrollment error = %v", err)
	}
	restarted, err := Open(ctx, databaseURL, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	tokenScope := sha256.Sum256([]byte(invalidToken))
	sourceScope := sha256.Sum256([]byte(throttledSource))
	if err := restarted.AllowEnrollmentAttempt(ctx, tokenScope, sourceScope, time.Now().UTC()); !errors.Is(err, devices.ErrEnrollmentRateLimited) {
		t.Fatalf("persisted throttle after repository restart = %v", err)
	}
}

func enrollmentIntegrationCSR(t *testing.T) []byte {
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

func integrationCertificate(issued devicepki.IssuedCertificate) devices.Certificate {
	return devices.Certificate{
		DeviceID: issued.DeviceID, Serial: issued.Serial, Fingerprint: issued.Fingerprint,
		PEM: issued.PEM, NotBefore: issued.NotBefore, NotAfter: issued.NotAfter,
	}
}
