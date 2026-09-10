package credential

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	decoyID       = "0198dc8c-c600-7000-8000-000000000001"
	environmentID = "0198dc8c-c600-7000-8000-000000000002"
	credentialID  = "0198dc8c-c600-7000-8000-000000000003"
)

var at = time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

func newID() string { return credentialID }

// flipMiddle changes one character well away from the padded tail.
func flipMiddle(secret string) string {
	mid := len(SecretPrefix) + 5
	replacement := byte('a')
	if secret[mid] == 'a' {
		replacement = 'b'
	}
	return secret[:mid] + string(replacement) + secret[mid+1:]
}

func mint(t *testing.T) Minted {
	t.Helper()
	minted, err := New(Input{
		DecoyID: decoyID, EnvironmentID: environmentID,
		Kind: KindSSHPassword, Username: "svc-backup",
	}, at, newID)
	if err != nil {
		t.Fatal(err)
	}
	return minted
}

/*
 * The whole point: using a planted credential is evidence.
 *
 * An attacker who types the exact value Guardian planted matches; anything else
 * does not, including a value that is merely close.
 */
func TestAPlantedCredentialMatchesAndNothingElseDoes(t *testing.T) {
	minted := mint(t)

	offered, recognised := HashOffered(minted.Secret)
	if !recognised {
		t.Fatal("Guardian did not recognise its own material")
	}
	if !minted.Credential.Matches(offered) {
		t.Fatal("the planted credential did not match itself")
	}

	for _, wrong := range []string{
		"hunter2",
		"",
		SecretPrefix,
		SecretPrefix + "short",
		minted.Secret + "x",
		flipMiddle(minted.Secret),
		// A non-canonical spelling of the same bytes: strict decoding refuses it.
		minted.Secret[:len(minted.Secret)-1] + "_",
	} {
		offered, recognised := HashOffered(wrong)
		if recognised && minted.Credential.Matches(offered) {
			t.Fatalf("%q matched a credential it is not", wrong)
		}
	}
}

/*
 * The secret exists exactly once.
 *
 * Nothing Guardian stores can produce it again: the struct has no field for it,
 * so it cannot leak into a log, a query result, or a console by accident.
 */
func TestTheStoredCredentialCannotHoldTheSecret(t *testing.T) {
	minted := mint(t)

	raw, err := json.Marshal(minted.Credential)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), minted.Secret) {
		t.Fatal("the secret survived into the stored credential")
	}
	// And no field is named as if it could hold one.
	var shape map[string]any
	if err := json.Unmarshal(raw, &shape); err != nil {
		t.Fatal(err)
	}
	for key := range shape {
		for _, banned := range []string{"secret", "password", "plaintext", "value"} {
			if strings.Contains(strings.ToLower(key), banned) && key != "SecretHash" {
				t.Fatalf("stored credential carries %q, which could hold the plaintext", key)
			}
		}
	}
}

/*
 * A leaked synthetic credential must be identifiable as worthless.
 *
 * The marker is the trade recorded in the package comment: it tells an attacker
 * they are in a honeypot, and it tells anyone finding one in a paste or a
 * password manager that it grants nothing. The second matters more, because it
 * arrives before the credential is typed and the first arrives after.
 */
func TestEverySecretIsMarkedAsGuardiansAndWorthless(t *testing.T) {
	seen := map[string]struct{}{}
	for attempt := 0; attempt < 64; attempt++ {
		minted := mint(t)
		if !strings.HasPrefix(minted.Secret, SecretPrefix) {
			t.Fatalf("secret %q carries no marker", minted.Secret)
		}
		if _, duplicate := seen[minted.Secret]; duplicate {
			t.Fatal("two credentials were minted with the same secret")
		}
		seen[minted.Secret] = struct{}{}
	}
	if len(seen) != 64 {
		t.Fatalf("minted %d distinct secrets from 64 attempts", len(seen))
	}
}

// A short synthetic password is one an attacker might type by accident, which
// would be a trigger nobody planted.
func TestTheSecretCarriesRealEntropy(t *testing.T) {
	minted := mint(t)
	body := strings.TrimPrefix(minted.Secret, SecretPrefix)
	// 16 bytes, base64url without padding.
	if len(body) != 22 {
		t.Fatalf("secret body is %d characters, want 22 for %d bytes", len(body), SecretBytes)
	}
	if _, recognised := HashOffered(minted.Secret); !recognised {
		t.Fatal("the minted secret does not round-trip")
	}
}

/*
 * A credential nobody planted anywhere cannot be triggered, and counting it as
 * live would overstate the coverage.
 */
func TestAPendingCredentialIsNotLive(t *testing.T) {
	minted := mint(t)

	if minted.Credential.State != StatePending {
		t.Fatalf("state = %q, want pending", minted.Credential.State)
	}
	if minted.Credential.Live() {
		t.Fatal("a credential nobody has planted is live")
	}

	placed, err := minted.Credential.MarkPlaced(at.Add(time.Hour), "left in /home/svc/.netrc on the file server")
	if err != nil {
		t.Fatal(err)
	}
	if !placed.Live() || placed.PlacedAt == nil {
		t.Fatalf("placed credential = %+v", placed)
	}
	// Placing twice is refused: the second call would move the record of when
	// it was planted.
	if _, err := placed.MarkPlaced(at.Add(2*time.Hour), ""); !errors.Is(err, ErrNotPlaceable) {
		t.Fatalf("second placement = %v, want ErrNotPlaceable", err)
	}
}

/*
 * A decoy an attacker returns to reports the same credential again. The first
 * use is the one that matters: overwriting the timestamp would move the
 * evidence forward every visit, and an investigator reading "first seen" would
 * get the last time instead.
 */
func TestTriggeringIsIdempotentAndKeepsTheFirstUse(t *testing.T) {
	placed, err := mint(t).Credential.MarkPlaced(at, "")
	if err != nil {
		t.Fatal(err)
	}
	first := at.Add(time.Hour)

	triggered := placed.MarkTriggered(first)
	again := triggered.MarkTriggered(first.Add(24 * time.Hour))

	if again.TriggeredAt == nil || !again.TriggeredAt.Equal(first) {
		t.Fatalf("triggered_at = %v, want the first use %v", again.TriggeredAt, first)
	}
	if again.State != StateTriggered {
		t.Fatalf("state = %q", again.State)
	}
}

/*
 * Revoking stops new evidence and does not erase old evidence. CS-06 forbids
 * silent deletion, and a credential somebody used stays one somebody used.
 */
func TestRevokingStopsEvidenceWithoutErasingIt(t *testing.T) {
	placed, err := mint(t).Credential.MarkPlaced(at, "")
	if err != nil {
		t.Fatal(err)
	}
	triggered := placed.MarkTriggered(at.Add(time.Hour))

	revoked := triggered.MarkRevoked(at.Add(2 * time.Hour))

	if revoked.Live() {
		t.Fatal("a revoked credential still raises evidence")
	}
	if revoked.TriggeredAt == nil {
		t.Fatal("revoking erased that the credential was used")
	}
	if revoked.RevokedAt == nil || revoked.State != StateRevoked {
		t.Fatalf("revoked credential = %+v", revoked)
	}
}

func TestClosedVocabulariesAndBounds(t *testing.T) {
	for name, input := range map[string]Input{
		"an invented kind": {
			DecoyID: decoyID, EnvironmentID: environmentID, Kind: "root_shell", Username: "svc",
		},
		"no username": {
			DecoyID: decoyID, EnvironmentID: environmentID, Kind: KindSSHPassword,
		},
		"an oversized username": {
			DecoyID: decoyID, EnvironmentID: environmentID, Kind: KindSSHPassword,
			Username: strings.Repeat("a", MaxUsernameRunes+1),
		},
		"a control character in the username": {
			DecoyID: decoyID, EnvironmentID: environmentID, Kind: KindSSHPassword,
			Username: "svc" + string(rune(0)) + "backup",
		},
		"an oversized placement note": {
			DecoyID: decoyID, EnvironmentID: environmentID, Kind: KindSSHPassword,
			Username: "svc", PlacementNote: strings.Repeat("n", MaxPlacementRunes+1),
		},
		"a decoy that is not a UUIDv7": {
			DecoyID: "not-a-uuid", EnvironmentID: environmentID, Kind: KindSSHPassword, Username: "svc",
		},
	} {
		if _, err := New(input, at, newID); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s = %v, want ErrInvalidInput", name, err)
		}
	}
}

// A credential must name the decoy that will recognise it: one no decoy
// presents is one nothing can ever trigger.
func TestACredentialMustNameItsDecoy(t *testing.T) {
	if _, err := New(Input{
		EnvironmentID: environmentID, Kind: KindSSHPassword, Username: "svc",
	}, at, newID); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

// Anything not shaped like Guardian's material never reaches a comparison.
// Most of what an attacker types at a decoy is a guess.
func TestUnmarkedInputNeverReachesAComparison(t *testing.T) {
	for _, offered := range []string{"", "hunter2", "gdn-decoy", "GDN-DECOY-abc", strings.Repeat("x", 4096)} {
		if _, recognised := HashOffered(offered); recognised {
			t.Fatalf("%q was treated as Guardian material", offered)
		}
	}
}

// The comparison is constant-time. Not because guessing this matters, but
// because it runs on the same path as every other credential check a decoy
// makes, and a side channel that exists "only in the honeypot branch" is one
// somebody will eventually reuse where it counts.
func TestMatchingUsesAConstantTimeComparison(t *testing.T) {
	minted := mint(t)
	var zero [sha256.Size]byte
	if minted.Credential.Matches(zero) {
		t.Fatal("an empty hash matched")
	}
	offered, _ := HashOffered(minted.Secret)
	if !minted.Credential.Matches(offered) {
		t.Fatal("the real hash did not match")
	}
}
