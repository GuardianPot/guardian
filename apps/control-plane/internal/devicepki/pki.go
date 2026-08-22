// Package devicepki contains the disposable CA and device identity fixture
// used to validate the Phase 0 device PKI boundary.
package devicepki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

var (
	ErrChallengeRequired = errors.New("valid enrollment challenge is required")
	ErrChallengeConsumed = errors.New("enrollment challenge was already consumed")
	ErrInvalidProof      = errors.New("enrollment proof is invalid")
	ErrDeviceExists      = errors.New("device already has an active certificate")
	ErrUnknownDevice     = errors.New("device is unknown")
	ErrRevoked           = errors.New("certificate is revoked")
	ErrNotActive         = errors.New("certificate is not the active device certificate")
)

const (
	challengeTTL   = 2 * time.Minute
	certificateTTL = 24 * time.Hour
)

// EnrollmentChallenge is a one-time server challenge for device enrollment.
type EnrollmentChallenge struct {
	DeviceID  string
	Nonce     []byte
	ExpiresAt time.Time
}

// EnrollmentRequest contains a CSR and proof that its private key holder saw
// the server-issued nonce.
type EnrollmentRequest struct {
	DeviceID  string
	Nonce     []byte
	CSR       *x509.CertificateRequest
	Signature []byte
}

type challengeState struct {
	nonce     []byte
	expiresAt time.Time
}

type deviceState struct {
	currentSerial string
}

// Authority is an in-memory, test-only product CA. It intentionally has no
// file-backed key store and must not be used as a production CA service.
type Authority struct {
	mu         sync.Mutex
	caKey      *ecdsa.PrivateKey
	caCert     *x509.Certificate
	roots      *x509.CertPool
	challenges map[string]challengeState
	devices    map[string]deviceState
	issued     map[string]string
	revoked    map[string]struct{}
}

// NewTestAuthority creates a disposable P-256 CA and trust pool.
func NewTestAuthority() (*Authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate test CA key: %w", err)
	}
	now := time.Now()
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Guardian P0-W9 Test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(certificateTTL),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create test CA certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse test CA certificate: %w", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	return &Authority{
		caKey:      key,
		caCert:     cert,
		roots:      roots,
		challenges: make(map[string]challengeState),
		devices:    make(map[string]deviceState),
		issued:     make(map[string]string),
		revoked:    make(map[string]struct{}),
	}, nil
}

// IssueChallenge invalidates any previous challenge for the device.
func (a *Authority) IssueChallenge(deviceID string) (EnrollmentChallenge, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return EnrollmentChallenge{}, err
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return EnrollmentChallenge{}, fmt.Errorf("generate enrollment nonce: %w", err)
	}
	expiresAt := time.Now().Add(challengeTTL)
	a.mu.Lock()
	a.challenges[deviceID] = challengeState{nonce: append([]byte(nil), nonce...), expiresAt: expiresAt}
	a.mu.Unlock()
	return EnrollmentChallenge{DeviceID: deviceID, Nonce: nonce, ExpiresAt: expiresAt}, nil
}

// NewDeviceRequest generates a device key, CSR, and nonce proof. The caller
// retains the private key; the authority never receives it.
func NewDeviceRequest(deviceID string, nonce []byte) (*ecdsa.PrivateKey, EnrollmentRequest, error) {
	if err := validateDeviceID(deviceID); err != nil {
		return nil, EnrollmentRequest{}, err
	}
	if len(nonce) < 16 {
		return nil, EnrollmentRequest{}, errors.New("enrollment nonce must contain at least 16 bytes")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, EnrollmentRequest{}, fmt.Errorf("generate device key: %w", err)
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: deviceID},
	}, key)
	if err != nil {
		return nil, EnrollmentRequest{}, fmt.Errorf("create device CSR: %w", err)
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, EnrollmentRequest{}, fmt.Errorf("parse device CSR: %w", err)
	}
	digest := proofDigest(deviceID, nonce, csr.Raw)
	signature, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return nil, EnrollmentRequest{}, fmt.Errorf("sign enrollment proof: %w", err)
	}
	return key, EnrollmentRequest{
		DeviceID:  deviceID,
		Nonce:     append([]byte(nil), nonce...),
		CSR:       csr,
		Signature: signature,
	}, nil
}

// Enroll signs the first active certificate for a device and consumes the
// challenge only after CSR and proof validation succeeds.
func (a *Authority) Enroll(request EnrollmentRequest) (*x509.Certificate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.validateRequestLocked(request); err != nil {
		return nil, err
	}
	if state, ok := a.devices[request.DeviceID]; ok && state.currentSerial != "" {
		if _, revoked := a.revoked[state.currentSerial]; !revoked {
			return nil, ErrDeviceExists
		}
	}
	return a.issueDeviceCertificateLocked(request, "")
}

// Rotate issues a new certificate and revokes the previous active certificate
// atomically from the authority's point of view.
func (a *Authority) Rotate(deviceID string, previous *x509.Certificate, request EnrollmentRequest) (*x509.Certificate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if request.DeviceID != deviceID || previous == nil || previous.Subject.CommonName != deviceID {
		return nil, ErrUnknownDevice
	}
	if err := a.verifyCertificateLocked(previous); err != nil {
		return nil, err
	}
	return a.issueDeviceCertificateLocked(request, serialKey(previous.SerialNumber))
}

// Revoke invalidates a certificate serial and is idempotent for an already
// revoked certificate belonging to the named device.
func (a *Authority) Revoke(deviceID string, cert *x509.Certificate) error {
	if cert == nil || cert.Subject.CommonName != deviceID {
		return ErrUnknownDevice
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     a.roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return ErrUnknownDevice
	}
	owner, ok := a.issued[serialKey(cert.SerialNumber)]
	if !ok || owner != deviceID {
		return ErrUnknownDevice
	}
	a.revoked[serialKey(cert.SerialNumber)] = struct{}{}
	return nil
}

// VerifyCertificate validates the CA chain, device identity, active serial,
// and revocation state.
func (a *Authority) VerifyCertificate(cert *x509.Certificate) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.verifyCertificateLocked(cert)
}

func (a *Authority) verifyCertificateLocked(cert *x509.Certificate) error {
	if cert == nil {
		return ErrUnknownDevice
	}
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       a.roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		CurrentTime: time.Now(),
	}); err != nil {
		return fmt.Errorf("verify device certificate chain: %w", err)
	}
	serial := serialKey(cert.SerialNumber)
	if _, revoked := a.revoked[serial]; revoked {
		return ErrRevoked
	}
	owner, issued := a.issued[serial]
	if !issued || owner != cert.Subject.CommonName {
		return ErrUnknownDevice
	}
	state, ok := a.devices[owner]
	if !ok {
		return ErrUnknownDevice
	}
	if state.currentSerial != serial {
		return ErrNotActive
	}
	return nil
}

// IssueServerIdentity creates a test-only server certificate signed by the
// same disposable CA for the mTLS handshake fixture.
func (a *Authority) IssueServerIdentity(host string) (tls.Certificate, error) {
	if host == "" {
		return tls.Certificate{}, errors.New("server host must not be empty")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate server key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(certificateTTL),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.caCert, &key.PublicKey, a.caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create server certificate: %w", err)
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal server key: %w", err)
	}
	return tls.X509KeyPair(pemEncode("CERTIFICATE", der), pemEncode("EC PRIVATE KEY", privateKey))
}

// ClientTLSConfig returns a TLS 1.3 client configuration for an enrolled
// device. It requires the server certificate to chain to the disposable CA.
func (a *Authority) ClientTLSConfig(cert *x509.Certificate, key *ecdsa.PrivateKey, serverName string) (*tls.Config, error) {
	clientCertificate, err := certificateForTLS(cert, key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		RootCAs:      a.roots,
		Certificates: []tls.Certificate{clientCertificate},
		ServerName:   serverName,
	}, nil
}

// ServerTLSConfig returns a TLS 1.3 server configuration that requires and
// verifies a currently active device certificate.
func (a *Authority) ServerTLSConfig(serverCertificate tls.Certificate) *tls.Config {
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{serverCertificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    a.roots,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return ErrUnknownDevice
			}
			return a.VerifyCertificate(state.PeerCertificates[0])
		},
	}
}

func (a *Authority) issueDeviceCertificateLocked(request EnrollmentRequest, previousSerial string) (*x509.Certificate, error) {
	if err := a.validateRequestLocked(request); err != nil {
		return nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: request.DeviceID},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(certificateTTL),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.caCert, request.CSR.PublicKey, a.caKey)
	if err != nil {
		return nil, fmt.Errorf("issue device certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse issued device certificate: %w", err)
	}
	delete(a.challenges, request.DeviceID)
	serialID := serialKey(cert.SerialNumber)
	if previousSerial != "" {
		a.revoked[previousSerial] = struct{}{}
	}
	a.devices[request.DeviceID] = deviceState{currentSerial: serialID}
	a.issued[serialID] = request.DeviceID
	return cert, nil
}

func (a *Authority) validateRequestLocked(request EnrollmentRequest) error {
	if err := validateDeviceID(request.DeviceID); err != nil {
		return err
	}
	challenge, ok := a.challenges[request.DeviceID]
	if !ok {
		return ErrChallengeConsumed
	}
	if time.Now().After(challenge.expiresAt) {
		delete(a.challenges, request.DeviceID)
		return ErrChallengeRequired
	}
	if !bytes.Equal(challenge.nonce, request.Nonce) {
		return ErrInvalidProof
	}
	if request.CSR == nil || request.CSR.Subject.CommonName != request.DeviceID {
		return ErrInvalidProof
	}
	if err := request.CSR.CheckSignature(); err != nil {
		return fmt.Errorf("%w: CSR signature: %v", ErrInvalidProof, err)
	}
	publicKey, ok := request.CSR.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("%w: device key must be ECDSA", ErrInvalidProof)
	}
	digest := proofDigest(request.DeviceID, request.Nonce, request.CSR.Raw)
	if !ecdsa.VerifyASN1(publicKey, digest[:], request.Signature) {
		return ErrInvalidProof
	}
	return nil
}

func certificateForTLS(cert *x509.Certificate, key *ecdsa.PrivateKey) (tls.Certificate, error) {
	if cert == nil || key == nil {
		return tls.Certificate{}, errors.New("certificate and private key are required")
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal device key: %w", err)
	}
	return tls.X509KeyPair(pemEncode("CERTIFICATE", cert.Raw), pemEncode("EC PRIVATE KEY", privateKey))
}

func proofDigest(deviceID string, nonce, csr []byte) [32]byte {
	hash := sha256.New()
	writePart := func(part []byte) {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(part)))
		hash.Write(length[:])
		hash.Write(part)
	}
	writePart([]byte(deviceID))
	writePart(nonce)
	writePart(csr)
	var digest [32]byte
	copy(digest[:], hash.Sum(nil))
	return digest
}

func randomSerial() (*big.Int, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial: %w", err)
	}
	return serial, nil
}

func serialKey(serial *big.Int) string {
	if serial == nil {
		return ""
	}
	return strings.ToLower(serial.Text(16))
}

func validateDeviceID(deviceID string) error {
	if deviceID == "" || len(deviceID) > 128 || strings.ContainsAny(deviceID, "\x00\r\n") {
		return errors.New("device ID must be non-empty, <=128 bytes, and line-safe")
	}
	return nil
}

func pemEncode(kind string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: der})
}
