package audit

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	cursor, err := NewCursor(9223372036854775807, 42)
	if err != nil {
		t.Fatalf("NewCursor: %v", err)
	}
	encoded, err := cursor.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded) > MaxCursorEncodedBytes {
		t.Fatalf("encoded length = %d", len(encoded))
	}
	for _, character := range encoded {
		if character == '=' || character == '+' || character == '/' {
			t.Fatalf("cursor is not unpadded Base64URL: %q", encoded)
		}
	}
	parsed, err := ParseCursor(encoded)
	if err != nil {
		t.Fatalf("ParseCursor: %v", err)
	}
	if parsed != cursor {
		t.Fatalf("parsed = %#v, want %#v", parsed, cursor)
	}
}

func TestCursorRejectsInvalidAndNonCanonicalInput(t *testing.T) {
	encode := func(raw string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(raw))
	}
	tests := map[string]string{
		"empty":            "",
		"invalid base64":   "***",
		"padded":           base64.URLEncoding.EncodeToString([]byte(`{"v":1,"a":10,"p":9}`)),
		"version":          encode(`{"v":2,"a":10,"p":9}`),
		"zero anchor":      encode(`{"v":1,"a":0,"p":0}`),
		"zero position":    encode(`{"v":1,"a":10,"p":0}`),
		"position forward": encode(`{"v":1,"a":10,"p":11}`),
		"unknown field":    encode(`{"v":1,"a":10,"p":9,"x":1}`),
		"duplicate field":  encode(`{"v":1,"a":10,"p":9,"p":9}`),
		"whitespace":       encode(`{ "v":1,"a":10,"p":9}`),
		"trailing JSON":    encode(`{"v":1,"a":10,"p":9}{}`),
	}
	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCursor(encoded); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("error = %v, want ErrInvalidCursor", err)
			}
		})
	}
}

func TestCursorZeroIsOnlyAnAbsentQueryCursor(t *testing.T) {
	if !(Cursor{}).IsZero() {
		t.Fatal("zero cursor not recognized")
	}
	if err := (Cursor{}).Validate(); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("zero cursor Validate error = %v", err)
	}
	if err := (ListQuery{}).Validate(); err != nil {
		t.Fatalf("first-page query rejected: %v", err)
	}
	if _, err := NewCursor(5, 6); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("forward cursor error = %v", err)
	}
}
