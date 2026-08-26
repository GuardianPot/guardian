// Package secretstore protects Control Plane secret records behind a stable
// authenticated-encryption boundary.
package secretstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	MasterKeyBytes  = 32
	envelopeVersion = byte(1)
)

var (
	ErrInvalidKey      = errors.New("secret store master key is invalid")
	ErrUnsafeKeyFile   = errors.New("secret store master key file is unsafe")
	ErrInvalidEnvelope = errors.New("secret store envelope is invalid")
)

// Store seals and opens bounded secret records using caller-supplied context
// as AEAD additional authenticated data.
type Store interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(envelope, context []byte) ([]byte, error)
}

// Local is the on-premises development backend. The mounted master key never
// enters the database, logs, or returned values.
type Local struct {
	aead cipher.AEAD
}

// LoadLocal reads exactly one owner-only, regular, non-symlink 32-byte key.
func LoadLocal(path string) (*Local, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("load secret store master key: %w", ErrInvalidKey)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Mode().Perm()&0o077 != 0 {
		return nil, ErrUnsafeKeyFile
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open secret store master key: %w", ErrInvalidKey)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return nil, ErrUnsafeKeyFile
	}
	key, err := io.ReadAll(io.LimitReader(file, MasterKeyBytes+1))
	if err != nil || len(key) != MasterKeyBytes {
		clear(key)
		return nil, ErrInvalidKey
	}
	store, err := NewLocal(key)
	clear(key)
	return store, err
}

// NewLocal constructs the backend from a transient key buffer. Tests and
// hosted adapters can use it without creating a plaintext key file.
func NewLocal(key []byte) (*Local, error) {
	if len(key) != MasterKeyBytes {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("construct secret store AEAD: %w", err)
	}
	return &Local{aead: aead}, nil
}

// Seal returns version || random nonce || authenticated ciphertext.
func (s *Local) Seal(plaintext, context []byte) ([]byte, error) {
	if s == nil || s.aead == nil || len(plaintext) == 0 {
		return nil, ErrInvalidEnvelope
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate secret store nonce: %w", err)
	}
	envelope := make([]byte, 1, 1+len(nonce)+len(plaintext)+s.aead.Overhead())
	envelope[0] = envelopeVersion
	envelope = append(envelope, nonce...)
	envelope = s.aead.Seal(envelope, nonce, plaintext, context)
	return envelope, nil
}

// Open authenticates the envelope version, nonce, ciphertext, and context.
func (s *Local) Open(envelope, context []byte) ([]byte, error) {
	if s == nil || s.aead == nil || len(envelope) < 1+s.aead.NonceSize()+s.aead.Overhead() || envelope[0] != envelopeVersion {
		return nil, ErrInvalidEnvelope
	}
	nonce := envelope[1 : 1+s.aead.NonceSize()]
	plaintext, err := s.aead.Open(nil, nonce, envelope[1+s.aead.NonceSize():], context)
	if err != nil {
		return nil, ErrInvalidEnvelope
	}
	return plaintext, nil
}
