// Package credential owns synthetic credentials: material Guardian plants so
// that using it is, by itself, evidence.
//
// The word "credential" is doing something unusual here. Everywhere else in this
// system a credential is something to protect. Here it is bait — deliberately
// discoverable, deliberately worthless, and valuable only because nobody with a
// legitimate reason would ever type it.
//
// That inverts the usual handling in one direction and not the other. It is
// fine for an attacker to find one; it is not fine for one to work anywhere
// real, and it is not fine for Guardian to keep the plaintext lying around
// after the operator has placed it. Both of those are enforced here.
package credential

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidInput = errors.New("synthetic credential input is invalid")
	ErrNotFound     = errors.New("synthetic credential was not found")
	// ErrAlreadyRevealed enforces the one-time rule. The secret is shown once,
	// at creation, and never again: AC "normal UI does not reveal reusable
	// secret after intended workflow" is a property of the store, not of the
	// screen that happens to be rendering it.
	ErrAlreadyRevealed = errors.New("synthetic credential secret was already revealed")
	ErrNotPlaceable    = errors.New("synthetic credential is not in a placeable state")
)

// Kind is what an attacker would think they had found.
//
// Closed, because each value implies a decoy family that knows how to present
// it. There is no generic value: a credential nothing can present is a
// credential nothing will ever trigger.
type Kind string

const (
	KindSSHPassword      Kind = "ssh_password"
	KindPostgresPassword Kind = "postgres_password"
	KindSMBPassword      Kind = "smb_password"
	KindHTTPBasic        Kind = "http_basic"
)

func Kinds() []Kind {
	return []Kind{KindSSHPassword, KindPostgresPassword, KindSMBPassword, KindHTTPBasic}
}

func (k Kind) Valid() bool {
	for _, candidate := range Kinds() {
		if candidate == k {
			return true
		}
	}
	return false
}

// State is where a planted credential is in its life.
//
// `triggered` is terminal in the sense that matters: a credential somebody used
// stays triggered, because the evidence that it was used does not stop being
// true. Revoking one afterwards records that Guardian stopped honouring it, not
// that the trigger was undone.
type State string

const (
	// StatePending exists between creation and the operator confirming they
	// placed it. A credential nobody planted anywhere cannot be triggered, and
	// counting it as live would overstate the coverage.
	StatePending   State = "pending"
	StatePlaced    State = "placed"
	StateTriggered State = "triggered"
	StateRevoked   State = "revoked"
)

func States() []State {
	return []State{StatePending, StatePlaced, StateTriggered, StateRevoked}
}

const (
	// SecretBytes is the entropy behind one credential. Sixteen bytes is beyond
	// any offline guessing concern for a value that grants nothing, and the
	// reason it is not smaller is that a short synthetic password is one an
	// attacker might type by accident, which would be a false trigger.
	SecretBytes = 16
	// MaxUsernameRunes bounds the account name an operator chooses. It is
	// attacker-visible by design, like a decoy persona.
	MaxUsernameRunes = 64
	MaxUsernameBytes = 256
	// MaxPlacementRunes bounds the note recording where the operator put it.
	MaxPlacementRunes = 256
	MaxPlacementBytes = 1024
)

/*
SecretPrefix marks every synthetic secret as Guardian's.

This is a deliberate trade and worth stating plainly. A marker makes a leaked
credential identifiable — anyone finding one in a paste, a log, or a repository
can tell it grants nothing and came from a deception system — and it lets
Guardian recognise its own material without a lookup.

It also tells an attacker who has found one that they are inside a honeypot.
That is the cost, and it is accepted for two reasons: the alternative is a
credential Guardian cannot distinguish from a real leaked one, which is far
worse the first time somebody finds one in a production password manager; and
an attacker who has already typed it has already generated the evidence, so the
tell arrives after the signal it would have suppressed.
*/
const SecretPrefix = "gdn-decoy-"

// Credential is one planted credential, as Guardian stores it.
//
// There is no plaintext field. The secret exists exactly once, in the value
// returned by New, and is never written here: a struct with nowhere to put it
// cannot leak one into a log, a query result, or a console.
type Credential struct {
	CredentialID string
	// DecoyID is which decoy will recognise this credential. Required: a
	// credential no decoy presents is one nothing can ever trigger.
	DecoyID       string
	EnvironmentID string
	Kind          Kind
	Username      string
	// SecretHash is what the decoy compares against. SHA-256 of the raw secret
	// bytes, matching the enrollment-token handling this repository already
	// reviewed.
	SecretHash [sha256.Size]byte
	State      State
	// PlacementNote is where the operator says they put it. Operator-authored
	// free text and untrusted to any renderer.
	PlacementNote string
	CreatedAt     time.Time
	PlacedAt      *time.Time
	TriggeredAt   *time.Time
	RevokedAt     *time.Time
	// Revealed records that the one-time disclosure has happened. Set by the
	// store when the secret is handed over, so a second attempt fails.
	Revealed bool
}

// Minted is a newly created credential and the one moment its secret exists.
//
// The caller shows the secret to the operator and then drops it. Nothing
// persists it, and there is no path that can produce it again.
type Minted struct {
	Credential Credential
	// Secret is the value an operator plants. Prefixed, so a leaked one is
	// identifiable as worthless.
	Secret string
}

// Input is what an operator supplies.
type Input struct {
	DecoyID       string
	EnvironmentID string
	Kind          Kind
	Username      string
	PlacementNote string
}

/*
New mints a credential.

The secret is generated here and returned once. It is hashed immediately and the
plaintext is not retained by anything this function keeps: the only copy that
outlives the call is the one the caller was handed, which is the copy the
operator plants.
*/
func New(input Input, now time.Time, newID func() string) (Minted, error) {
	if !ValidUUIDv7(input.DecoyID) || !ValidUUIDv7(input.EnvironmentID) {
		return Minted{}, fmt.Errorf("%w: decoy and environment must be UUIDv7", ErrInvalidInput)
	}
	if !input.Kind.Valid() {
		return Minted{}, fmt.Errorf("%w: kind %q is outside the closed set", ErrInvalidInput, input.Kind)
	}
	username, err := normalizeBounded(input.Username, MaxUsernameRunes, MaxUsernameBytes, "username")
	if err != nil {
		return Minted{}, err
	}
	note, err := normalizeOptional(input.PlacementNote, MaxPlacementRunes, MaxPlacementBytes, "placement note")
	if err != nil {
		return Minted{}, err
	}

	raw := make([]byte, SecretBytes)
	if _, err := rand.Read(raw); err != nil {
		// No fallback. A predictable synthetic credential is worse than none:
		// it would produce triggers nobody planted.
		return Minted{}, fmt.Errorf("%w: entropy source failed", ErrInvalidInput)
	}
	secret := SecretPrefix + base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256(raw)
	clear(raw)

	return Minted{
		Credential: Credential{
			CredentialID: newID(), DecoyID: input.DecoyID, EnvironmentID: input.EnvironmentID,
			Kind: input.Kind, Username: username, SecretHash: hash,
			State: StatePending, PlacementNote: note, CreatedAt: now.UTC(),
		},
		Secret: secret,
	}, nil
}

/*
HashOffered turns a credential an attacker typed into something comparable.

Returns false when the value is not shaped like Guardian's material at all,
which is the common case and is not an error: most of what an attacker types at
a decoy is a guess, and only a planted credential should reach a comparison.
*/
func HashOffered(offered string) ([sha256.Size]byte, bool) {
	rest, found := strings.CutPrefix(offered, SecretPrefix)
	if !found {
		return [sha256.Size]byte{}, false
	}
	// Strict: base64 leaves spare bits in the final character of a 16-byte
	// encoding, so a lenient decoder accepts several textual forms of the same
	// credential. One credential must have exactly one spelling, or "the
	// attacker typed the value we planted" stops being a precise claim.
	raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
	if err != nil || len(raw) != SecretBytes {
		clear(raw)
		return [sha256.Size]byte{}, false
	}
	hash := sha256.Sum256(raw)
	clear(raw)
	return hash, true
}

/*
Matches reports whether an offered credential is this one.

Constant-time, and not because an attacker guessing a synthetic credential
matters — it does not, the thing grants nothing. It is constant-time because
the comparison runs on the same code path as every other credential check a
decoy makes, and a timing side channel that exists "only in the honeypot branch"
is a side channel someone will eventually reuse somewhere it counts.
*/
func (c Credential) Matches(offered [sha256.Size]byte) bool {
	return subtle.ConstantTimeCompare(c.SecretHash[:], offered[:]) == 1
}

// Placeable reports whether the credential may still be planted.
func (c Credential) Placeable() bool { return c.State == StatePending }

// Live reports whether a use of this credential should still raise evidence.
//
// A revoked credential does not. That is the point of revoking one: an operator
// who tore down the location they planted it in should stop being paged when
// somebody eventually finds the stale copy.
func (c Credential) Live() bool { return c.State == StatePlaced }

// MarkPlaced records that the operator planted it.
func (c Credential) MarkPlaced(at time.Time, note string) (Credential, error) {
	if !c.Placeable() {
		return Credential{}, fmt.Errorf("%w: state is %q", ErrNotPlaceable, c.State)
	}
	cleaned, err := normalizeOptional(note, MaxPlacementRunes, MaxPlacementBytes, "placement note")
	if err != nil {
		return Credential{}, err
	}
	placed := at.UTC()
	c.State, c.PlacedAt = StatePlaced, &placed
	if cleaned != "" {
		c.PlacementNote = cleaned
	}
	return c, nil
}

/*
MarkTriggered records that somebody used it.

Idempotent, and deliberately: a decoy an attacker returns to will report the
same credential again, and the first use is the one that matters. Overwriting
the timestamp would move the evidence forward every time they came back, and an
investigator reading "first seen" would get the last time instead.
*/
func (c Credential) MarkTriggered(at time.Time) Credential {
	if c.TriggeredAt == nil {
		triggered := at.UTC()
		c.TriggeredAt = &triggered
	}
	if c.State == StatePending || c.State == StatePlaced {
		c.State = StateTriggered
	}
	return c
}

// MarkRevoked stops the credential raising new evidence.
//
// It does not erase that it was triggered. `CS-06` forbids silent deletion, and
// a credential somebody used stays a credential somebody used.
func (c Credential) MarkRevoked(at time.Time) Credential {
	revoked := at.UTC()
	c.RevokedAt = &revoked
	c.State = StateRevoked
	return c
}

func normalizeBounded(value string, maxRunes, maxBytes int, field string) (string, error) {
	cleaned, err := normalizeOptional(value, maxRunes, maxBytes, field)
	if err != nil {
		return "", err
	}
	if cleaned == "" {
		return "", fmt.Errorf("%w: %s is required", ErrInvalidInput, field)
	}
	return cleaned, nil
}

// normalizeOptional trims, bounds, and rejects control characters.
//
// The same rules the decoy display name follows, for the same reason: these are
// attacker-visible strings an operator typed, and they travel to a console.
func normalizeOptional(value string, maxRunes, maxBytes int, field string) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalidInput, field)
	}
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) > maxRunes || len(trimmed) > maxBytes {
		return "", fmt.Errorf("%w: %s exceeds %d code points", ErrInvalidInput, field, maxRunes)
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: %s contains a control character", ErrInvalidInput, field)
		}
	}
	return trimmed, nil
}

// ValidUUIDv7 accepts only the canonical lowercase form every Guardian identity
// uses.
func ValidUUIDv7(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' ||
		value[14] != '7' {
		return false
	}
	switch value[19] {
	case '8', '9', 'a', 'b':
	default:
		return false
	}
	for index, character := range []byte(value) {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') {
			continue
		}
		return false
	}
	return true
}
