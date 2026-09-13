/*
Package syntheticcred is the Edge's half of the synthetic credential domain
(`P2-W9` section 5): the entry a workload definition carries for each credential
planted for its decoy, and the rule that recognises one when an attacker types
it.

Two processes use it and neither may widen it. The privileged helper validates
the entries when it reads a definition; the Edge Agent will recognise offered
values against them once a pack presents credentials (`P2-W5`, `P2-W7`). The
package therefore reads no file, opens no socket, and imports nothing from
either side: it is a pure function of a definition's bytes and an offered
string, and `TestThePackageReachesNothing` holds it to that.

The recognition rule is the Control Plane's `credential.HashOffered` and
`Credential.Matches`, byte for byte. The two live in different modules and
cannot share code, so both modules pin the same fixture — one secret, its hash,
and its entry — and a change to either rule fails the other's test.

What an entry carries is a SHA-256 of 128 bits of worthless random material.
Never the secret: a workload definition is world-readable on the host, and the
value an attacker typed is not retained here beyond the one comparison.
*/
package syntheticcred

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind is what an attacker would think they had found. The closed set is the
// Control Plane's `credential.Kinds()`.
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

const (
	// SecretPrefix and SecretBytes are the Control Plane's. See
	// `credential.SecretPrefix` for why the marker exists.
	SecretPrefix = "gdn-decoy-"
	SecretBytes  = 16

	// MaxEntries bounds how many credentials one decoy recognises. Every
	// offered value is compared against all of them, so the bound is also the
	// bound on the work an attacker can cause per attempt.
	MaxEntries = 16

	// MaxUsernameRunes and MaxUsernameBytes are the Control Plane's bounds.
	MaxUsernameRunes = 64
	MaxUsernameBytes = 256
)

// ErrInvalidEntry never carries the offending value. The fields are not secret,
// but an error is the kind of thing that reaches a log, and the rule is simpler
// to keep when no value from a definition ever does.
var ErrInvalidEntry = errors.New("synthetic credential entry is invalid")

// Entry is one planted credential as a workload definition carries it. The
// Control Plane renders it (`Credential.WorkloadEntry`) and an operator installs
// it; the field names and forms are pinned by the shared fixture.
type Entry struct {
	CredentialID string `json:"credential_id"`
	Kind         Kind   `json:"kind"`
	Username     string `json:"username"`
	SecretSHA256 string `json:"secret_sha256"`
}

// ValidateEntries checks a definition's entries as a whole: the count, every
// field's form, and that no credential or hash appears twice.
//
// A duplicate hash is refused as well as a duplicate id, because two ids for one
// secret would make "which credential did the attacker use" ambiguous, and the
// answer is the whole of the evidence.
func ValidateEntries(entries []Entry) error {
	_, err := parse(entries)
	return err
}

type entry struct {
	id   string
	kind Kind
	hash [sha256.Size]byte
}

func parse(entries []Entry) ([]entry, error) {
	if len(entries) > MaxEntries {
		return nil, invalid("too many entries")
	}
	parsed := make([]entry, 0, len(entries))
	ids := make(map[string]struct{}, len(entries))
	hashes := make(map[[sha256.Size]byte]struct{}, len(entries))
	for _, e := range entries {
		if !ValidUUIDv7(e.CredentialID) {
			return nil, invalid("credential_id is not a canonical UUIDv7")
		}
		if !e.Kind.Valid() {
			return nil, invalid("kind is outside the closed set")
		}
		if !validUsername(e.Username) {
			return nil, invalid("username is empty, untrimmed, oversized, or contains a control character")
		}
		hash, ok := parseHash(e.SecretSHA256)
		if !ok {
			return nil, invalid("secret_sha256 is not 64 lowercase hex characters")
		}
		if _, duplicate := ids[e.CredentialID]; duplicate {
			return nil, invalid("credential_id appears twice")
		}
		if _, duplicate := hashes[hash]; duplicate {
			return nil, invalid("secret_sha256 appears twice")
		}
		ids[e.CredentialID], hashes[hash] = struct{}{}, struct{}{}
		parsed = append(parsed, entry{id: e.CredentialID, kind: e.Kind, hash: hash})
	}
	return parsed, nil
}

// invalid takes only a constant reason, never a value from the definition.
func invalid(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidEntry, reason) }

// The Control Plane trims the username before storing it, so the one form an
// entry may carry is the trimmed one. Anything else was edited by hand and is
// refused rather than repaired.
func validUsername(value string) bool {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	if len(value) > MaxUsernameBytes || utf8.RuneCountInString(value) > MaxUsernameRunes {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// parseHash accepts exactly the Control Plane's rendering: lowercase hex, no
// prefix. hex.DecodeString alone would also accept upper case, which is a second
// spelling of the same entry.
func parseHash(value string) ([sha256.Size]byte, bool) {
	var hash [sha256.Size]byte
	if len(value) != hex.EncodedLen(sha256.Size) {
		return hash, false
	}
	for _, character := range []byte(value) {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return hash, false
		}
	}
	if _, err := hex.Decode(hash[:], []byte(value)); err != nil {
		return [sha256.Size]byte{}, false
	}
	return hash, true
}

/*
HashOffered is `credential.HashOffered`, and must stay identical to it.

It returns false when the value is not shaped like Guardian's material, which is
the common case and not an error. Strict unpadded URL base64: the final
character of a 16-byte encoding carries spare bits, and a lenient decoder would
give one credential several spellings. Exactly 22 characters: even a strict
decoder skips CR and LF, so a secret followed by a newline would otherwise be a
second spelling.
*/
func HashOffered(offered string) ([sha256.Size]byte, bool) {
	rest, found := strings.CutPrefix(offered, SecretPrefix)
	if !found || len(rest) != base64.RawURLEncoding.EncodedLen(SecretBytes) {
		return [sha256.Size]byte{}, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
	if err != nil || len(raw) != SecretBytes {
		clear(raw)
		return [sha256.Size]byte{}, false
	}
	hash := sha256.Sum256(raw)
	clear(raw)
	return hash, true
}

// Set is a validated group of entries, ready to recognise against.
//
// It holds ids, kinds, and hashes, and has no field that could hold an offered
// value: what an attacker typed is compared and dropped.
type Set struct {
	entries []entry
}

// NewSet validates entries and prepares them for recognition.
func NewSet(entries []Entry) (Set, error) {
	parsed, err := parse(entries)
	if err != nil {
		return Set{}, err
	}
	return Set{entries: parsed}, nil
}

// Len is how many credentials the set recognises.
func (s Set) Len() int { return len(s.entries) }

/*
Recognize reports which planted credential of the given kind an offered value
is, if any.

Every entry is compared, with a constant-time comparison and no early exit,
whether or not an earlier one matched: the time taken depends on the number of
entries and on nothing an attacker typed beyond whether it carried Guardian's
marker at all. That is `credential.Matches`, applied across the set. Kind is
compared the same way, so a PostgreSQL credential offered to SSH is not
recognised and does not take a different path to say so.

The returned id is what a pack's adapter puts in `auth.synthetic_credential_id`.
The offered value goes nowhere.
*/
func (s Set) Recognize(kind Kind, offered string) (string, bool) {
	hash, shaped := HashOffered(offered)
	if !shaped {
		return "", false
	}
	found, index := 0, 0
	for position, candidate := range s.entries {
		sameHash := subtle.ConstantTimeCompare(candidate.hash[:], hash[:])
		sameKind := subtle.ConstantTimeCompare([]byte(candidate.kind), []byte(kind))
		hit := sameHash & sameKind
		index = subtle.ConstantTimeSelect(hit, position, index)
		found |= hit
	}
	clear(hash[:])
	if found != 1 {
		return "", false
	}
	return s.entries[index].id, true
}

// ValidUUIDv7 accepts only the canonical lowercase form every Guardian identity
// uses; the same rule as the Control Plane and `normalize.ValidUUIDv7`.
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
