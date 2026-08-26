package devicepki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/secretstore"
)

const (
	DeviceCertificateLifetime = 30 * 24 * time.Hour
	RotationWindow            = 10 * 24 * time.Hour
	caEnvelopeContext         = "guardian.device-ca.v1"
	maxCSRBytes               = 32 << 10
)

var (
	ErrInvalidCSR         = errors.New("device certificate request is invalid")
	ErrInvalidDeviceID    = errors.New("device identity is invalid")
	ErrInvalidCAMaterial  = errors.New("device CA material is invalid")
	ErrCertificateInvalid = errors.New("device certificate is invalid")
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Material is the persistable CA boundary. PrivateKeyEnvelope is always
// authenticated ciphertext produced by SecretStore.
type Material struct {
	CertificatePEM     []byte
	PrivateKeyEnvelope []byte
	NotAfter           time.Time
}

// IssuedCertificate contains only certificate/public metadata.
type IssuedCertificate struct {
	DeviceID    string
	Serial      string
	Fingerprint string
	PEM         []byte
	NotBefore   time.Time
	NotAfter    time.Time
}

// ProductAuthority owns the decrypted in-process signing key. It never exposes
// or persists plaintext private-key bytes.
type ProductAuthority struct {
	certificate *x509.Certificate
	privateKey  *ecdsa.PrivateKey
	roots       *x509.CertPool
}

// GenerateMaterial creates a new product CA for the explicit initialization
// command and seals the PKCS#8 key before returning.
func GenerateMaterial(secrets secretstore.Store, now time.Time) (Material, error) {
	if secrets == nil {
		return Material{}, ErrInvalidCAMaterial
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return Material{}, fmt.Errorf("generate device CA key: %w", err)
	}
	serial, err := randomSerial()
	if err != nil {
		return Material{}, err
	}
	now = now.UTC()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Guardian Device CA"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return Material{}, fmt.Errorf("create device CA certificate: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return Material{}, fmt.Errorf("marshal device CA key: %w", err)
	}
	defer clear(privateDER)
	envelope, err := secrets.Seal(privateDER, []byte(caEnvelopeContext))
	if err != nil {
		return Material{}, fmt.Errorf("seal device CA key: %w", err)
	}
	return Material{
		CertificatePEM:     pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		PrivateKeyEnvelope: envelope,
		NotAfter:           template.NotAfter.UTC(),
	}, nil
}

// LoadProductAuthority authenticates and opens persisted CA material.
func LoadProductAuthority(material Material, secrets secretstore.Store, now time.Time) (*ProductAuthority, error) {
	if secrets == nil || len(material.CertificatePEM) == 0 || len(material.PrivateKeyEnvelope) == 0 {
		return nil, ErrInvalidCAMaterial
	}
	block, rest := pem.Decode(material.CertificatePEM)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return nil, ErrInvalidCAMaterial
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !certificate.IsCA || now.UTC().Before(certificate.NotBefore) || !now.UTC().Before(certificate.NotAfter) {
		return nil, ErrInvalidCAMaterial
	}
	privateDER, err := secrets.Open(material.PrivateKeyEnvelope, []byte(caEnvelopeContext))
	if err != nil {
		return nil, ErrInvalidCAMaterial
	}
	defer clear(privateDER)
	parsed, err := x509.ParsePKCS8PrivateKey(privateDER)
	if err != nil {
		return nil, ErrInvalidCAMaterial
	}
	privateKey, ok := parsed.(*ecdsa.PrivateKey)
	publicKey, publicOK := certificate.PublicKey.(*ecdsa.PublicKey)
	if !ok || !publicOK || privateKey.Curve != elliptic.P256() || !publicKey.Equal(&privateKey.PublicKey) {
		return nil, ErrInvalidCAMaterial
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	return &ProductAuthority{certificate: certificate, privateKey: privateKey, roots: roots}, nil
}

// Issue validates proof of key possession and issues one bounded client cert.
func (a *ProductAuthority) Issue(deviceID string, csrPEM []byte, now time.Time) (IssuedCertificate, error) {
	if a == nil || a.certificate == nil || a.privateKey == nil {
		return IssuedCertificate{}, ErrInvalidCAMaterial
	}
	if !uuidV7Pattern.MatchString(deviceID) {
		return IssuedCertificate{}, ErrInvalidDeviceID
	}
	if len(csrPEM) == 0 || len(csrPEM) > maxCSRBytes {
		return IssuedCertificate{}, ErrInvalidCSR
	}
	block, rest := pem.Decode(csrPEM)
	if block == nil || block.Type != "CERTIFICATE REQUEST" || len(rest) != 0 {
		return IssuedCertificate{}, ErrInvalidCSR
	}
	request, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || request.CheckSignature() != nil {
		return IssuedCertificate{}, ErrInvalidCSR
	}
	if request.SignatureAlgorithm != x509.ECDSAWithSHA256 ||
		len(request.Subject.Names) != 0 || len(request.Subject.ExtraNames) != 0 ||
		len(request.Extensions) != 0 || len(request.ExtraExtensions) != 0 || len(request.Attributes) != 0 ||
		len(request.DNSNames) != 0 || len(request.EmailAddresses) != 0 ||
		len(request.IPAddresses) != 0 || len(request.URIs) != 0 {
		return IssuedCertificate{}, ErrInvalidCSR
	}
	publicKey, ok := request.PublicKey.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return IssuedCertificate{}, ErrInvalidCSR
	}
	serial, err := randomSerial()
	if err != nil {
		return IssuedCertificate{}, err
	}
	identityURI, _ := url.Parse("urn:guardian:device:" + deviceID)
	now = now.UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{},
		URIs:         []*url.URL{identityURI},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(DeviceCertificateLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.certificate, publicKey, a.privateKey)
	if err != nil {
		return IssuedCertificate{}, fmt.Errorf("issue device certificate: %w", err)
	}
	fingerprint := sha256.Sum256(der)
	return IssuedCertificate{
		DeviceID:    deviceID,
		Serial:      serial.Text(16),
		Fingerprint: hex.EncodeToString(fingerprint[:]),
		PEM:         pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		NotBefore:   template.NotBefore.UTC(),
		NotAfter:    template.NotAfter.UTC(),
	}, nil
}

// Verify parses the device UUID URI SAN and validates the client chain.
func (a *ProductAuthority) Verify(certificate *x509.Certificate, now time.Time) (string, error) {
	if a == nil || certificate == nil {
		return "", ErrCertificateInvalid
	}
	if _, err := certificate.Verify(x509.VerifyOptions{
		Roots:       a.roots,
		CurrentTime: now.UTC(),
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		return "", ErrCertificateInvalid
	}
	if len(certificate.URIs) != 1 || certificate.URIs[0].Scheme != "urn" {
		return "", ErrCertificateInvalid
	}
	const prefix = "urn:guardian:device:"
	text := certificate.URIs[0].String()
	if len(text) <= len(prefix) || text[:len(prefix)] != prefix || !uuidV7Pattern.MatchString(text[len(prefix):]) {
		return "", ErrCertificateInvalid
	}
	return text[len(prefix):], nil
}

// CertificatePEM returns a defensive copy of the public CA certificate.
func (a *ProductAuthority) CertificatePEM() []byte {
	if a == nil || a.certificate == nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: a.certificate.Raw})
}

func parseCertificate(certificatePEM []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(certificatePEM)
	if block == nil || block.Type != "CERTIFICATE" || len(rest) != 0 {
		return nil, ErrCertificateInvalid
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, ErrCertificateInvalid
	}
	return certificate, nil
}

// ParseCertificate exposes strict parsing for the storage and TLS boundaries.
func ParseCertificate(certificatePEM []byte) (*x509.Certificate, error) {
	return parseCertificate(certificatePEM)
}

// RotationDue reports whether now has entered the final ten-day window.
func RotationDue(notAfter, now time.Time) bool {
	return !now.UTC().Before(notAfter.UTC().Add(-RotationWindow))
}

// Ensure the production serial generator retains the 128-bit P0-W9 contract.
func randomProductSerial() (*big.Int, error) { return randomSerial() }
