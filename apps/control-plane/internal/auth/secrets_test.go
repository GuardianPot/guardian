package auth

import (
	"testing"
	"time"
)

func TestOpaqueAndRecoverySecretsAreBoundedAndHashable(t *testing.T) {
	first, firstHash, err := GenerateOpaqueSecret()
	if err != nil {
		t.Fatal(err)
	}
	second, secondHash, err := GenerateOpaqueSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != OpaqueSecretLength || first == second || firstHash == secondHash {
		t.Fatalf("opaque secret contract failed: lengths=%d/%d equal=%t", len(first), len(second), first == second)
	}
	parsed, err := HashOpaqueSecret(first)
	if err != nil || parsed != firstHash {
		t.Fatalf("HashOpaqueSecret = %x, %v", parsed, err)
	}
	codes, hashes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != RecoveryCodeCount || len(hashes) != RecoveryCodeCount {
		t.Fatalf("recovery counts = %d/%d", len(codes), len(hashes))
	}
	seen := map[string]struct{}{}
	for index, code := range codes {
		if len(code) != RecoveryCodeLength {
			t.Fatalf("recovery code length = %d", len(code))
		}
		if _, exists := seen[code]; exists {
			t.Fatal("duplicate recovery code")
		}
		seen[code] = struct{}{}
		parsed, err := HashRecoveryCode(code)
		if err != nil || parsed != hashes[index] {
			t.Fatalf("HashRecoveryCode = %x, %v", parsed, err)
		}
	}
}

func TestUUIDv7CarriesVersionAndVariant(t *testing.T) {
	value, err := NewUUIDv7(time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(value) != 36 || value[14] != '7' || !containsByte("89ab", value[19]) {
		t.Fatalf("UUIDv7 = %q", value)
	}
}

func containsByte(value string, candidate byte) bool {
	for index := range value {
		if value[index] == candidate {
			return true
		}
	}
	return false
}
