package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPasswordHashRoundTripAndRuntimeProfile(t *testing.T) {
	password, err := NormalizeNewPassword("correct horse battery staple", "owner")
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	encoded, err := HashPassword(password, DefaultArgon2Params)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed >= time.Second {
		t.Fatalf("default Argon2id profile took %s, want below one second", elapsed)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("PHC = %q", encoded)
	}
	valid, err := VerifyPassword(password, encoded)
	if err != nil || !valid {
		t.Fatalf("VerifyPassword(valid) = %t, %v", valid, err)
	}
	valid, err = VerifyPassword("incorrect horse battery staple", encoded)
	if err != nil || valid {
		t.Fatalf("VerifyPassword(invalid) = %t, %v", valid, err)
	}
}

func TestPasswordPolicyNormalizesNFCAndBlocksWeakValues(t *testing.T) {
	decomposed := "mot-de-passe-e\u0301-tres-long"
	normalized, err := NormalizeNewPassword(decomposed, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(normalized, "e\u0301") {
		t.Fatalf("password was not NFC normalized: %q", normalized)
	}
	for _, value := range []string{"short", "password123!", strings.Repeat("a", MaximumPasswordRunes+1)} {
		if _, err := NormalizeNewPassword(value, "owner"); !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("NormalizeNewPassword(%q) error = %v", value, err)
		}
	}
}

func TestPasswordRejectsMalformedWeakOrOversizedPHCBeforeAllocation(t *testing.T) {
	values := []string{
		"not-a-phc",
		"$argon2i$v=19$m=65536,t=3,p=4$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU",
		"$argon2id$v=19$m=19456,t=2,p=1$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU",
		"$argon2id$v=19$m=9999999999,t=3,p=4$YWJjZGVmZ2hpamtsbW5vcA$YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU",
	}
	for _, value := range values {
		if _, err := VerifyPassword("correct horse battery staple", value); !errors.Is(err, ErrInvalidHash) {
			t.Fatalf("VerifyPassword(%q) error = %v", value, err)
		}
	}
}
