package environment

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeName(t *testing.T) {
	name, err := NormalizeName("  Cafe\u0301  ")
	if err != nil {
		t.Fatal(err)
	}
	if name.DisplayName != "Café" || name.NameKey != "café" {
		t.Fatalf("NormalizeName() = %+v", name)
	}
	for _, value := range []string{"", "\n", strings.Repeat("a", MaxNameRunes+1), string([]byte{0xff})} {
		if _, err := NormalizeName(value); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("NormalizeName(%q) error = %v", value, err)
		}
	}
	left, err := NormalizeName("Straße")
	if err != nil {
		t.Fatal(err)
	}
	right, err := NormalizeName("STRASSE")
	if err != nil || left.NameKey != right.NameKey {
		t.Fatalf("Unicode case-fold keys = %q and %q (error %v)", left.NameKey, right.NameKey, err)
	}
	maximum := strings.Repeat("😀", MaxNameRunes)
	if utf8.RuneCountInString(maximum) != MaxNameRunes || len(maximum) != MaxNameBytes {
		t.Fatal("test maximum does not exercise both name bounds")
	}
	if _, err := NormalizeName(maximum); err != nil {
		t.Fatalf("maximum bounded name error = %v", err)
	}
}

func TestNormalizePrivateIPv4Prefix(t *testing.T) {
	valid := []string{
		"10.0.0.0/8", "10.1.2.3/32", "172.16.0.0/12", "172.31.255.0/24",
		"192.168.0.0/16", "192.168.255.255/32",
	}
	for _, value := range valid {
		if actual, err := NormalizePrivateIPv4Prefix(value); err != nil || actual != value {
			t.Errorf("NormalizePrivateIPv4Prefix(%q) = (%q, %v)", value, actual, err)
		}
	}
	invalid := []string{
		"10.0.0.1/24", "9.255.255.0/24", "172.15.0.0/16", "172.32.0.0/16",
		"192.169.0.0/16", "127.0.0.0/8", "169.254.0.0/16", "224.0.0.0/4",
		"0.0.0.0/0", "2001:db8::/32", "not-a-prefix", " 10.0.0.0/8",
	}
	for _, value := range invalid {
		if _, err := NormalizePrivateIPv4Prefix(value); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("NormalizePrivateIPv4Prefix(%q) error = %v", value, err)
		}
	}
}
