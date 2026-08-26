package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const (
	OpaqueSecretBytes     = 32
	OpaqueSecretLength    = 43
	RecoveryCodeBytes     = 16
	RecoveryCodeLength    = 22
	RecoveryCodeCount     = 10
	BootstrapTokenExpiry  = 15 * time.Minute
	SessionIdleExpiry     = 15 * time.Minute
	SessionAbsoluteExpiry = 8 * time.Hour
)

var ErrInvalidSecret = errors.New("opaque secret is invalid")

func GenerateOpaqueSecret() (string, [sha256.Size]byte, error) {
	raw := make([]byte, OpaqueSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate opaque secret: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	clear(raw)
	return encoded, hash, nil
}

func HashOpaqueSecret(encoded string) ([sha256.Size]byte, error) {
	if len(encoded) != OpaqueSecretLength {
		return [sha256.Size]byte{}, ErrInvalidSecret
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != OpaqueSecretBytes {
		clear(raw)
		return [sha256.Size]byte{}, ErrInvalidSecret
	}
	hash := sha256.Sum256(raw)
	clear(raw)
	return hash, nil
}

func GenerateRecoveryCodes() ([]string, [][sha256.Size]byte, error) {
	codes := make([]string, RecoveryCodeCount)
	hashes := make([][sha256.Size]byte, RecoveryCodeCount)
	for index := range codes {
		raw := make([]byte, RecoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		codes[index] = base64.RawURLEncoding.EncodeToString(raw)
		hashes[index] = sha256.Sum256(raw)
		clear(raw)
	}
	return codes, hashes, nil
}

func HashRecoveryCode(encoded string) ([sha256.Size]byte, error) {
	if len(encoded) != RecoveryCodeLength {
		return [sha256.Size]byte{}, ErrInvalidSecret
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(raw) != RecoveryCodeBytes {
		clear(raw)
		return [sha256.Size]byte{}, ErrInvalidSecret
	}
	hash := sha256.Sum256(raw)
	clear(raw)
	return hash, nil
}
