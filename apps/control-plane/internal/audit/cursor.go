package audit

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	CursorVersion         = 1
	MaxCursorEncodedBytes = 128
)

var ErrInvalidCursor = errors.New("invalid audit cursor")

// Cursor carries both the immutable first-page sequence anchor and the last
// position returned. The public representation is an opaque, versioned,
// unpadded Base64URL string.
type Cursor struct {
	AnchorSequence   int64
	PositionSequence int64
}

type cursorEnvelope struct {
	Version  int   `json:"v"`
	Anchor   int64 `json:"a"`
	Position int64 `json:"p"`
}

// NewCursor constructs a validated pagination cursor.
func NewCursor(anchorSequence, positionSequence int64) (Cursor, error) {
	cursor := Cursor{AnchorSequence: anchorSequence, PositionSequence: positionSequence}
	if err := cursor.Validate(); err != nil {
		return Cursor{}, err
	}
	return cursor, nil
}

// IsZero reports whether the cursor represents an absent first-page cursor.
func (c Cursor) IsZero() bool {
	return c.AnchorSequence == 0 && c.PositionSequence == 0
}

// Validate rejects partial, non-positive, or forward-moving cursors.
func (c Cursor) Validate() error {
	if c.AnchorSequence <= 0 {
		return fmt.Errorf("%w: anchor sequence must be positive", ErrInvalidCursor)
	}
	if c.PositionSequence <= 0 {
		return fmt.Errorf("%w: position sequence must be positive", ErrInvalidCursor)
	}
	if c.PositionSequence > c.AnchorSequence {
		return fmt.Errorf("%w: position sequence exceeds anchor", ErrInvalidCursor)
	}
	return nil
}

// Encode returns the canonical unpadded Base64URL representation.
func (c Cursor) Encode() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	raw, err := json.Marshal(cursorEnvelope{
		Version:  CursorVersion,
		Anchor:   c.AnchorSequence,
		Position: c.PositionSequence,
	})
	if err != nil {
		return "", fmt.Errorf("%w: encode envelope: %v", ErrInvalidCursor, err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	if len(encoded) > MaxCursorEncodedBytes {
		return "", fmt.Errorf("%w: encoded cursor exceeds %d bytes", ErrInvalidCursor, MaxCursorEncodedBytes)
	}
	return encoded, nil
}

// ParseCursor strictly validates version, fields, Base64URL form, and the
// anchor/position relationship. Non-canonical JSON and padded encodings fail.
func ParseCursor(encoded string) (Cursor, error) {
	if encoded == "" {
		return Cursor{}, fmt.Errorf("%w: cursor is empty", ErrInvalidCursor)
	}
	if len(encoded) > MaxCursorEncodedBytes {
		return Cursor{}, fmt.Errorf("%w: encoded cursor exceeds %d bytes", ErrInvalidCursor, MaxCursorEncodedBytes)
	}
	if strings.Contains(encoded, "=") {
		return Cursor{}, fmt.Errorf("%w: padding is not permitted", ErrInvalidCursor)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return Cursor{}, fmt.Errorf("%w: invalid Base64URL", ErrInvalidCursor)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var envelope cursorEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return Cursor{}, fmt.Errorf("%w: invalid envelope", ErrInvalidCursor)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Cursor{}, fmt.Errorf("%w: trailing data", ErrInvalidCursor)
	}
	if envelope.Version != CursorVersion {
		return Cursor{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidCursor, envelope.Version)
	}
	cursor, err := NewCursor(envelope.Anchor, envelope.Position)
	if err != nil {
		return Cursor{}, err
	}
	canonical, err := cursor.Encode()
	if err != nil {
		return Cursor{}, err
	}
	if canonical != encoded {
		return Cursor{}, fmt.Errorf("%w: non-canonical encoding", ErrInvalidCursor)
	}
	return cursor, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("additional JSON value")
		}
		return err
	}
	return nil
}
