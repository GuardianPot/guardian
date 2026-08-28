// Package identity loads the protected Edge device identity without exposing
// private-key material to logs or diagnostics.
package identity

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const maxIdentityFileBytes = 64 << 10

var (
	// ErrUnavailable means the configured secure identity cannot be loaded.
	ErrUnavailable = errors.New("secure device identity is unavailable")
	// ErrPermissions means identity files do not meet the protected-file baseline.
	ErrPermissions = errors.New("device identity permissions are unsafe")
	// ErrInvalid means the certificate and key are malformed or do not match.
	ErrInvalid = errors.New("device identity is invalid")
	// ErrExpired means the certificate is outside its validity window.
	ErrExpired = errors.New("device identity certificate is not currently valid")
)

// Metadata is the non-secret subset safe to persist and expose in diagnostics.
type Metadata struct {
	DeviceID          string
	CertificateSerial string
	CertificateSHA256 string
	NotBefore         time.Time
	NotAfter          time.Time
}

// Credentials keeps the validated TLS keypair in memory for the outbound mTLS
// channel while exposing only Metadata to diagnostics and persistence.
type Credentials struct {
	Certificate tls.Certificate
	Leaf        *x509.Certificate
	Metadata    Metadata
}

// Load validates protected files, a matching keypair, and certificate validity.
// It never attempts enrollment or an insecure transport fallback.
func Load(certPath, keyPath string, now time.Time) (Metadata, error) {
	credentials, err := LoadCredentials(certPath, keyPath, now)
	if err != nil {
		return Metadata{}, err
	}
	return credentials.Metadata, nil
}

// LoadCredentials validates and loads the product device identity for the
// outbound channel. Private-key bytes are cleared after tls parses them.
func LoadCredentials(certPath, keyPath string, now time.Time) (Credentials, error) {
	if err := recoverInstall(certPath, keyPath); err != nil {
		return Credentials{}, err
	}
	certificatePEM, err := readIdentityFile(certPath, false)
	if err != nil {
		return Credentials{}, err
	}
	privateKeyPEM, err := readIdentityFile(keyPath, true)
	if err != nil {
		return Credentials{}, err
	}
	defer clear(privateKeyPEM)

	pair, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return Credentials{}, fmt.Errorf("%w: load matching certificate and key", ErrInvalid)
	}
	if len(pair.Certificate) == 0 {
		return Credentials{}, fmt.Errorf("%w: certificate chain is empty", ErrInvalid)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return Credentials{}, fmt.Errorf("%w: parse leaf certificate", ErrInvalid)
	}
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return Credentials{}, fmt.Errorf("%w: certificate validity window", ErrExpired)
	}
	deviceID, err := deviceIDFromCertificate(leaf)
	if err != nil {
		return Credentials{}, err
	}
	fingerprint := sha256.Sum256(leaf.Raw)
	pair.Leaf = leaf
	metadata := Metadata{
		DeviceID:          deviceID,
		CertificateSerial: strings.ToLower(leaf.SerialNumber.Text(16)),
		CertificateSHA256: hex.EncodeToString(fingerprint[:]),
		NotBefore:         leaf.NotBefore.UTC(),
		NotAfter:          leaf.NotAfter.UTC(),
	}
	return Credentials{Certificate: pair, Leaf: leaf, Metadata: metadata}, nil
}

func deviceIDFromCertificate(certificate *x509.Certificate) (string, error) {
	if len(certificate.URIs) != 1 {
		return "", fmt.Errorf("%w: certificate must contain one device URI SAN", ErrInvalid)
	}
	const prefix = "urn:guardian:device:"
	value := certificate.URIs[0].String()
	if !strings.HasPrefix(value, prefix) || !validUUIDv7(strings.TrimPrefix(value, prefix)) {
		return "", fmt.Errorf("%w: certificate device URI SAN", ErrInvalid)
	}
	return strings.TrimPrefix(value, prefix), nil
}

func validUUIDv7(value string) bool {
	if len(value) != 36 || value != strings.ToLower(value) || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' || value[14] != '7' || !strings.ContainsRune("89ab", rune(value[19])) {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func readIdentityFile(path string, private bool) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%w: identity file", ErrUnavailable)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: identity must be a regular non-symlink file", ErrPermissions)
	}
	if private && before.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%w: private key must be owner-only", ErrPermissions)
	}
	if !private && before.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("%w: certificate must not be group- or world-writable", ErrPermissions)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: open identity file", ErrUnavailable)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("%w: identity file changed while opening", ErrPermissions)
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxIdentityFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read identity file", ErrUnavailable)
	}
	if len(contents) > maxIdentityFileBytes {
		return nil, fmt.Errorf("%w: identity file exceeds %d bytes", ErrInvalid, maxIdentityFileBytes)
	}
	return contents, nil
}
