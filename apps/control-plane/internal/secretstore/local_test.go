package secretstore

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalRoundTripBindsContextAndNonce(t *testing.T) {
	store, err := NewLocal(bytes.Repeat([]byte{0x42}, MasterKeyBytes))
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Seal([]byte("ca-private-key"), []byte("guardian.device-ca.v1"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Seal([]byte("ca-private-key"), []byte("guardian.device-ca.v1"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("randomized envelopes are identical")
	}
	plain, err := store.Open(first, []byte("guardian.device-ca.v1"))
	if err != nil || string(plain) != "ca-private-key" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	if _, err := store.Open(first, []byte("wrong-context")); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("wrong context error = %v", err)
	}
	first[len(first)-1] ^= 0xff
	if _, err := store.Open(first, []byte("guardian.device-ca.v1")); !errors.Is(err, ErrInvalidEnvelope) {
		t.Fatalf("tamper error = %v", err)
	}
}

func TestLoadLocalRequiresOwnerOnlyRegularExactKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose the Unix owner-only permission contract")
	}
	root := t.TempDir()
	valid := filepath.Join(root, "master.key")
	if err := os.WriteFile(valid, bytes.Repeat([]byte{0x21}, MasterKeyBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocal(valid); err != nil {
		t.Fatalf("valid key: %v", err)
	}
	unsafe := filepath.Join(root, "unsafe.key")
	if err := os.WriteFile(unsafe, bytes.Repeat([]byte{0x21}, MasterKeyBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocal(unsafe); !errors.Is(err, ErrUnsafeKeyFile) {
		t.Fatalf("unsafe key error = %v", err)
	}
	short := filepath.Join(root, "short.key")
	if err := os.WriteFile(short, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLocal(short); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("short key error = %v", err)
	}
}
