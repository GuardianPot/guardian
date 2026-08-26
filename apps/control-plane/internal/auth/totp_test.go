package auth

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTOTPSHA256DriftAndReplayWindow(t *testing.T) {
	seed := bytes.Repeat([]byte{0x42}, TOTPSeedBytes)
	now := time.Unix(1_800_000_000, 0).UTC()
	code, err := TOTPCode(seed, now)
	if err != nil {
		t.Fatal(err)
	}
	counter, ok := VerifyTOTP(seed, code, now.Add(20*time.Second), -1)
	if !ok || counter != now.Unix()/30 {
		t.Fatalf("VerifyTOTP = %d, %t", counter, ok)
	}
	if _, ok := VerifyTOTP(seed, code, now, counter); ok {
		t.Fatal("replayed TOTP was accepted")
	}
	if _, ok := VerifyTOTP(seed, code, now.Add(2*TOTPStep), -1); ok {
		t.Fatal("TOTP outside drift window was accepted")
	}
}

func TestTOTPProvisioningURIContainsBoundedContract(t *testing.T) {
	seed := bytes.Repeat([]byte{0x11}, TOTPSeedBytes)
	uri, err := ProvisioningURI("Guardian", "owner@example.test", seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"otpauth://totp/Guardian:owner@example.test", "algorithm=SHA256", "digits=6", "period=30", "secret="} {
		if !strings.Contains(uri, expected) {
			t.Fatalf("URI %q does not contain %q", uri, expected)
		}
	}
}
