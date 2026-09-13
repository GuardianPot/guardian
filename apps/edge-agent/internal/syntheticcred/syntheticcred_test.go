package syntheticcred

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/GuardianPot/guardian/apps/edge-agent/internal/normalize"
)

/*
The shared fixture. `apps/control-plane/internal/credential` pins the same three
values; if either module's rule or rendering changes, one of the two tests fails.

The secret is the sixteen bytes 0x00..0x0f. It is a fixture, not bait: it is
public in this repository and must never be planted.
*/
const (
	fixtureSecret       = "gdn-decoy-AAECAwQFBgcICQoLDA0ODw"
	fixtureSecretSHA256 = "be45cb2605bf36bebde684841a28f0fd43c69850a3dce5fedba69928ee3a8991"
	fixtureEntry        = `{"credential_id":"0198dc8c-c600-7000-8000-000000000003","kind":"ssh_password","username":"svc-backup","secret_sha256":"be45cb2605bf36bebde684841a28f0fd43c69850a3dce5fedba69928ee3a8991"}`
	fixtureCredentialID = "0198dc8c-c600-7000-8000-000000000003"
)

func decodeEntry(t *testing.T, raw string) Entry {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entry Entry
	if err := decoder.Decode(&entry); err != nil {
		t.Fatal(err)
	}
	return entry
}

func fixtureSet(t *testing.T) Set {
	t.Helper()
	set, err := NewSet([]Entry{decodeEntry(t, fixtureEntry)})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestTheSharedFixtureIsTheControlPlanesRule(t *testing.T) {
	raw := make([]byte, SecretBytes)
	for index := range raw {
		raw[index] = byte(index)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != fixtureSecretSHA256 {
		t.Fatal("the fixture hash is not SHA-256 of the fixture bytes")
	}
	hash, shaped := HashOffered(fixtureSecret)
	if !shaped || hash != sum {
		t.Fatal("HashOffered does not reproduce the fixture hash")
	}

	entry := decodeEntry(t, fixtureEntry)
	rendered, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if string(rendered) != fixtureEntry {
		t.Fatalf("entry renders as %s, want the fixture", rendered)
	}

	id, recognised := fixtureSet(t).Recognize(KindSSHPassword, fixtureSecret)
	if !recognised || id != fixtureCredentialID {
		t.Fatalf("Recognize = %q, %v", id, recognised)
	}
}

// The closed set is the Control Plane's. A kind added on one side only would be
// a credential the other side cannot carry.
func TestKindsAreTheControlPlanesClosedSet(t *testing.T) {
	want := []Kind{"ssh_password", "postgres_password", "smb_password", "http_basic"}
	if !slices.Equal(Kinds(), want) {
		t.Fatalf("Kinds() = %v, want %v", Kinds(), want)
	}
}

/*
 * Only the exact planted value, offered to a decoy of its kind, is recognised.
 */
func TestOnlyThePlantedValueIsRecognised(t *testing.T) {
	set := fixtureSet(t)
	body := strings.TrimPrefix(fixtureSecret, SecretPrefix)
	for name, offered := range map[string]string{
		"a guess":                  "hunter2",
		"nothing":                  "",
		"the marker alone":         SecretPrefix,
		"the body without marker":  body,
		"the marker in upper case": strings.ToUpper(SecretPrefix) + body,
		"a trailing character":     fixtureSecret + "x",
		"a trailing newline":       fixtureSecret + "\n",
		"a trailing CRLF":          fixtureSecret + "\r\n",
		"a newline inside":         SecretPrefix + body[:11] + "\n" + body[11:],
		"a leading space":          " " + fixtureSecret,
		"one character changed":    SecretPrefix + "B" + body[1:],
		"standard base64 alphabet": SecretPrefix + strings.ReplaceAll(body, "_", "/") + "==",
		"padded":                   fixtureSecret + "==",
		// The final character carries spare bits; strict decoding refuses the
		// second spelling of the same bytes.
		"a non-canonical final character": fixtureSecret[:len(fixtureSecret)-1] + "x",
		"seventeen bytes":                 SecretPrefix + "AAECAwQFBgcICQoLDA0ODxA",
		"oversized":                       SecretPrefix + strings.Repeat("A", 4096),
	} {
		if id, recognised := set.Recognize(KindSSHPassword, offered); recognised {
			t.Fatalf("%s was recognised as %q", name, id)
		}
	}
	for _, kind := range []Kind{KindPostgresPassword, KindSMBPassword, KindHTTPBasic, "", "ssh_password "} {
		if _, recognised := set.Recognize(kind, fixtureSecret); recognised {
			t.Fatalf("the SSH credential was recognised when offered as %q", kind)
		}
	}
}

// Every entry is compared; the match can be anywhere in the set and the right
// id comes back.
func TestRecognitionFindsTheMatchingEntryAnywhereInTheSet(t *testing.T) {
	entries := make([]Entry, 0, MaxEntries)
	for index := range MaxEntries - 1 {
		hash := sha256.Sum256([]byte{byte(index)})
		entries = append(entries, Entry{
			CredentialID: fmt.Sprintf("0198dc8c-c600-7000-8000-0000000001%02x", index),
			Kind:         KindSSHPassword, Username: "svc", SecretSHA256: hex.EncodeToString(hash[:]),
		})
	}
	fixture := decodeEntry(t, fixtureEntry)
	for _, position := range []int{0, MaxEntries / 2, MaxEntries - 1} {
		arranged := slices.Insert(slices.Clone(entries), position, fixture)
		set, err := NewSet(arranged)
		if err != nil {
			t.Fatal(err)
		}
		if id, recognised := set.Recognize(KindSSHPassword, fixtureSecret); !recognised || id != fixtureCredentialID {
			t.Fatalf("at position %d: Recognize = %q, %v", position, id, recognised)
		}
	}
	if _, recognised := (Set{}).Recognize(KindSSHPassword, fixtureSecret); recognised {
		t.Fatal("an empty set recognised something")
	}
}

func TestAnEntryIsStrictlyFormed(t *testing.T) {
	valid := decodeEntry(t, fixtureEntry)
	for name, mutate := range map[string]func(*Entry){
		"no credential id":               func(e *Entry) { e.CredentialID = "" },
		"an upper-case credential id":    func(e *Entry) { e.CredentialID = strings.ToUpper(e.CredentialID) },
		"a UUIDv4":                       func(e *Entry) { e.CredentialID = "0198dc8c-c600-4000-8000-000000000003" },
		"an invented kind":               func(e *Entry) { e.Kind = "root_shell" },
		"no kind":                        func(e *Entry) { e.Kind = "" },
		"no username":                    func(e *Entry) { e.Username = "" },
		"an untrimmed username":          func(e *Entry) { e.Username = " svc-backup" },
		"a control character":            func(e *Entry) { e.Username = "svc\x00backup" },
		"invalid UTF-8":                  func(e *Entry) { e.Username = "svc\xff" },
		"an oversized username":          func(e *Entry) { e.Username = strings.Repeat("a", MaxUsernameRunes+1) },
		"no hash":                        func(e *Entry) { e.SecretSHA256 = "" },
		"an upper-case hash":             func(e *Entry) { e.SecretSHA256 = strings.ToUpper(e.SecretSHA256) },
		"a prefixed hash":                func(e *Entry) { e.SecretSHA256 = "sha256:" + e.SecretSHA256[:57] },
		"a short hash":                   func(e *Entry) { e.SecretSHA256 = e.SecretSHA256[:62] },
		"a long hash":                    func(e *Entry) { e.SecretSHA256 += "00" },
		"the secret instead of a hash":   func(e *Entry) { e.SecretSHA256 = fixtureSecret },
		"a hash with a non-hex letter":   func(e *Entry) { e.SecretSHA256 = "g" + e.SecretSHA256[1:] },
		"a hash with a space in it":      func(e *Entry) { e.SecretSHA256 = " " + e.SecretSHA256[1:] },
		"a hash that is 64 base64 runes": func(e *Entry) { e.SecretSHA256 = strings.Repeat("A", 64) },
	} {
		entry := valid
		mutate(&entry)
		if err := ValidateEntries([]Entry{entry}); !errors.Is(err, ErrInvalidEntry) {
			t.Fatalf("%s = %v, want ErrInvalidEntry", name, err)
		}
	}
}

func TestASetIsBoundedAndUnambiguous(t *testing.T) {
	valid := decodeEntry(t, fixtureEntry)
	if err := ValidateEntries(nil); err != nil {
		t.Fatalf("no entries = %v", err)
	}

	sameID := valid
	sameID.SecretSHA256 = strings.Repeat("0", 64)
	if err := ValidateEntries([]Entry{valid, sameID}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("a duplicate credential id = %v, want ErrInvalidEntry", err)
	}
	// Two ids for one secret would make "which credential was used" ambiguous.
	sameHash := valid
	sameHash.CredentialID = "0198dc8c-c600-7000-8000-000000000004"
	sameHash.Kind = KindPostgresPassword
	if err := ValidateEntries([]Entry{valid, sameHash}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("a duplicate hash = %v, want ErrInvalidEntry", err)
	}

	tooMany := make([]Entry, 0, MaxEntries+1)
	for index := range MaxEntries + 1 {
		hash := sha256.Sum256([]byte{byte(index)})
		tooMany = append(tooMany, Entry{
			CredentialID: fmt.Sprintf("0198dc8c-c600-7000-8000-0000000002%02x", index),
			Kind:         KindSSHPassword, Username: "svc", SecretSHA256: hex.EncodeToString(hash[:]),
		})
	}
	if err := ValidateEntries(tooMany); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("%d entries = %v, want ErrInvalidEntry", len(tooMany), err)
	}
	if err := ValidateEntries(tooMany[:MaxEntries]); err != nil {
		t.Fatalf("%d entries = %v", MaxEntries, err)
	}
}

// An error is the kind of thing that reaches a log, so none carries a value
// from the definition.
func TestAnErrorCarriesNoValue(t *testing.T) {
	entry := decodeEntry(t, fixtureEntry)
	entry.Username = "svc-backup\x00"
	err := ValidateEntries([]Entry{entry})
	if err == nil {
		t.Fatal("accepted")
	}
	for _, value := range []string{"svc-backup", fixtureSecretSHA256, fixtureCredentialID} {
		if strings.Contains(err.Error(), value) {
			t.Fatalf("error %q carries %q", err, value)
		}
	}
}

/*
 * The typed value never enters an event.
 *
 * What recognition hands a pack's adapter is an id; the `P2-W10` envelope field
 * it fills is that id. Neither the set, nor the result, nor the auth block it
 * produces holds what the attacker typed.
 */
func TestTheOfferedValueNeverReachesTheEnvelope(t *testing.T) {
	set := fixtureSet(t)
	body := strings.TrimPrefix(fixtureSecret, SecretPrefix)

	id, recognised := set.Recognize(KindSSHPassword, fixtureSecret)
	if !recognised {
		t.Fatal("not recognised")
	}
	identity := "svc-backup"
	auth := normalize.Auth{Identity: &identity, Result: "success", SyntheticCredentialID: &id}
	rendered, err := json.Marshal(auth)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(rendered, []byte(`"synthetic_credential_id":"`+fixtureCredentialID+`"`)) {
		t.Fatalf("auth = %s, want the credential id", rendered)
	}
	for _, surface := range []string{string(rendered), fmt.Sprintf("%+v", set), fmt.Sprintf("%#v", set)} {
		if strings.Contains(surface, body) {
			t.Fatalf("the offered value reached %q", surface)
		}
	}
}
