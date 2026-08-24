package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSnapshotBuildsVersionedEnvelopeAndRedactsRecursively(t *testing.T) {
	projection := map[string]any{
		"user_id":       "user-1",
		"password_hash": "hash",
		"token_id":      "safe-token-id",
		"nested": map[string]any{
			"Session-Token": "secret-token",
			"items": []any{
				map[string]any{"TOTP_seed": "seed", "private_key": "key"},
			},
		},
	}
	snapshot, err := NewSnapshot(projection)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	if snapshot.IsZero() {
		t.Fatal("constructed snapshot is zero")
	}

	var envelope struct {
		Schema string         `json:"schema"`
		Data   map[string]any `json:"data"`
	}
	if err := json.Unmarshal(snapshot.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Schema != SnapshotSchema {
		t.Fatalf("schema = %q", envelope.Schema)
	}
	if envelope.Data["password_hash"] != RedactedSnapshotValue {
		t.Fatalf("password_hash = %#v", envelope.Data["password_hash"])
	}
	if envelope.Data["token_id"] != "safe-token-id" {
		t.Fatalf("safe token_id was redacted: %#v", envelope.Data["token_id"])
	}
	nested := envelope.Data["nested"].(map[string]any)
	if nested["Session-Token"] != RedactedSnapshotValue {
		t.Fatalf("nested token = %#v", nested["Session-Token"])
	}
	item := nested["items"].([]any)[0].(map[string]any)
	if item["TOTP_seed"] != RedactedSnapshotValue || item["private_key"] != RedactedSnapshotValue {
		t.Fatalf("recursive secrets not redacted: %#v", item)
	}
	if _, err := ParseSnapshot(snapshot.Bytes()); err != nil {
		t.Fatalf("ParseSnapshot(builder output): %v", err)
	}
}

func TestKnownSecretKeyNormalizationIsExact(t *testing.T) {
	secretKeys := []string{
		"password", "password_hash", "session-token", "bootstrap_token",
		"enrollment.token", "recovery_token", "access_token", "refresh token",
		"totp_seed", "TOTP_SECRET", "totp-code", "csrf", "authorization",
		"cookie", "device_private_key", "ca-private-key", "private.key",
		"private_key_pem", "device_private_key_bytes", "ca_private_key_material",
		"hashed_password", "password_digest", "session_token_value", "passwords",
		"api_key", "api_secret_value", "client_secret_value", "mfa_secret_value",
		"mfa_code_value", "csrf_value", "authorization_header_value", "session_cookie_value",
		"password_hash_value", "password_digest_value",
		"auth_token", "bearer_token", "id_token", "secret_key", "session_token_encrypted",
		"private_key_pkcs8", "totp_seed_base64",
	}
	for _, key := range secretKeys {
		if !IsKnownSecretKey(key) {
			t.Errorf("secret key %q was not recognized", key)
		}
	}
	for _, key := range []string{
		"token_id", "session_token_id", "private_key_id", "private_key_fingerprint",
		"device_private_key_fingerprint", "password_id", "password_policy", "password_policy_version",
		"api_key_id", "client_secret_id", "session_cookie_id", "authorization_id",
		"auth_token_id", "id_token_id", "secret_key_fingerprint",
	} {
		if IsKnownSecretKey(key) {
			t.Errorf("safe ID key %q was over-redacted", key)
		}
	}
}

func TestSnapshotRedactsRealisticSecretVariantsButPreservesReferences(t *testing.T) {
	projection := map[string]any{
		"authentication": map[string]any{
			"hashed_password":            "argon2id-hash",
			"password_digest":            "digest",
			"password_hash_value":        "hash-value",
			"password_digest_value":      "digest-value",
			"passwords":                  []any{"old", "new"},
			"session_token_value":        "opaque-session-token",
			"mfa_secret_value":           "mfa-secret",
			"mfa_code_value":             "123456",
			"csrf_value":                 "csrf-secret",
			"authorization_header_value": "Bearer secret",
			"session_cookie_value":       "cookie-secret",
			"auth_token":                 "auth-token",
			"bearer_token":               "bearer-token",
			"id_token":                   "identity-token",
			"session_token_encrypted":    "encrypted-session-token",
			"totp_seed_base64":           "base64-seed",
		},
		"service_credentials": map[string]any{
			"api_key":             "api-key",
			"api_secret_value":    "api-secret",
			"client_secret_value": "client-secret",
			"secret_key":          "generic-secret-key",
		},
		"cryptography": []any{
			map[string]any{"private_key_pem": "pem-value"},
			map[string]any{"device_private_key_bytes": "private-bytes"},
			map[string]any{"ca_private_key_material": "ca-material"},
			map[string]any{"private_key_pkcs8": "pkcs8-material"},
		},
		"references": map[string]any{
			"token_id":                       "token-reference",
			"private_key_fingerprint":        "sha256:public-fingerprint",
			"device_private_key_fingerprint": "sha256:device-fingerprint",
			"password_policy":                "argon2id-v1",
			"api_key_id":                     "api-key-reference",
			"session_cookie_id":              "cookie-reference",
			"auth_token_id":                  "auth-token-reference",
			"secret_key_fingerprint":         "sha256:secret-key-fingerprint",
		},
	}
	snapshot, err := NewSnapshot(projection)
	if err != nil {
		t.Fatalf("NewSnapshot: %v", err)
	}
	if _, err := ParseSnapshot(snapshot.Bytes()); err != nil {
		t.Fatalf("ParseSnapshot(builder output): %v", err)
	}

	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(snapshot.Bytes(), &envelope); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	authentication := envelope.Data["authentication"].(map[string]any)
	for _, key := range []string{
		"hashed_password", "password_digest", "password_hash_value", "password_digest_value",
		"passwords", "session_token_value", "mfa_secret_value", "mfa_code_value",
		"csrf_value", "authorization_header_value", "session_cookie_value",
		"auth_token", "bearer_token", "id_token", "session_token_encrypted", "totp_seed_base64",
	} {
		if authentication[key] != RedactedSnapshotValue {
			t.Errorf("secret variant %q = %#v, want redacted", key, authentication[key])
		}
	}
	for key, value := range envelope.Data["service_credentials"].(map[string]any) {
		if value != RedactedSnapshotValue {
			t.Errorf("service credential %q = %#v, want redacted", key, value)
		}
	}
	for index, item := range envelope.Data["cryptography"].([]any) {
		for key, value := range item.(map[string]any) {
			if value != RedactedSnapshotValue {
				t.Errorf("cryptography item %d key %q = %#v, want redacted", index, key, value)
			}
		}
	}
	references := envelope.Data["references"].(map[string]any)
	wantReferences := map[string]any{
		"token_id":                       "token-reference",
		"private_key_fingerprint":        "sha256:public-fingerprint",
		"device_private_key_fingerprint": "sha256:device-fingerprint",
		"password_policy":                "argon2id-v1",
		"api_key_id":                     "api-key-reference",
		"session_cookie_id":              "cookie-reference",
		"auth_token_id":                  "auth-token-reference",
		"secret_key_fingerprint":         "sha256:secret-key-fingerprint",
	}
	for key, want := range wantReferences {
		if references[key] != want {
			t.Errorf("safe reference %q = %#v, want %#v", key, references[key], want)
		}
	}
}

func TestParseSnapshotRejectsUnredactedSecretVariants(t *testing.T) {
	for _, key := range []string{
		"private_key_pem",
		"device_private_key_bytes",
		"ca_private_key_material",
		"hashed_password",
		"password_digest",
		"session_token_value",
		"passwords",
		"api_key",
		"api_secret_value",
		"client_secret_value",
		"mfa_secret_value",
		"mfa_code_value",
		"csrf_value",
		"authorization_header_value",
		"session_cookie_value",
		"password_hash_value",
		"password_digest_value",
		"auth_token",
		"bearer_token",
		"id_token",
		"secret_key",
		"session_token_encrypted",
		"private_key_pkcs8",
		"totp_seed_base64",
	} {
		t.Run(key, func(t *testing.T) {
			encoded, err := json.Marshal(map[string]any{
				"schema": SnapshotSchema,
				"data": map[string]any{
					"nested": map[string]any{key: "plaintext"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseSnapshot(encoded); !errors.Is(err, ErrUnredactedSnapshotSecret) {
				t.Fatalf("error = %v, want ErrUnredactedSnapshotSecret", err)
			}
		})
	}
}

func TestSnapshotExactStructuralLimits(t *testing.T) {
	object64 := make(map[string]any, MaxSnapshotMembers)
	for i := 0; i < MaxSnapshotMembers; i++ {
		object64[fmt.Sprintf("k%02d", i)] = true
	}
	if _, err := NewSnapshot(object64); err != nil {
		t.Fatalf("64-member object rejected: %v", err)
	}
	object64["overflow"] = true
	if _, err := NewSnapshot(object64); !errors.Is(err, ErrSnapshotTooManyMembers) {
		t.Fatalf("65-member object error = %v", err)
	}

	if _, err := NewSnapshot(map[string]any{strings.Repeat("k", MaxSnapshotKeyBytes): true}); err != nil {
		t.Fatalf("64-byte key rejected: %v", err)
	}
	if _, err := NewSnapshot(map[string]any{strings.Repeat("k", MaxSnapshotKeyBytes+1): true}); !errors.Is(err, ErrSnapshotKeyTooLong) {
		t.Fatalf("65-byte key error = %v", err)
	}

	if _, err := NewSnapshot(map[string]any{"value": strings.Repeat("v", MaxSnapshotStringBytes)}); err != nil {
		t.Fatalf("512-byte string rejected: %v", err)
	}
	if _, err := NewSnapshot(map[string]any{"value": strings.Repeat("v", MaxSnapshotStringBytes+1)}); !errors.Is(err, ErrSnapshotStringTooLong) {
		t.Fatalf("513-byte string error = %v", err)
	}

	wrap := func(levels int) map[string]any {
		var value any = "leaf"
		for range levels {
			value = map[string]any{"child": value}
		}
		return value.(map[string]any)
	}
	depthBoundary, err := NewSnapshot(wrap(MaxSnapshotDepth - 1))
	if err != nil {
		t.Fatalf("depth %d rejected: %v", MaxSnapshotDepth, err)
	}
	if _, err := ParseSnapshot(depthBoundary.Bytes()); err != nil {
		t.Fatalf("stored depth %d rejected: %v", MaxSnapshotDepth, err)
	}
	if _, err := NewSnapshot(wrap(MaxSnapshotDepth)); !errors.Is(err, ErrSnapshotTooDeep) {
		t.Fatalf("depth %d error = %v", MaxSnapshotDepth+1, err)
	}

	makeNodeProjection := func(lastArrayMembers int) map[string]any {
		lengths := []int{63, 63, 62, lastArrayMembers}
		arrays := make([]any, len(lengths))
		for i, length := range lengths {
			values := make([]any, length)
			for j := range values {
				values[j] = true
			}
			arrays[i] = values
		}
		return map[string]any{"items": arrays}
	}
	nodeBoundary, err := NewSnapshot(makeNodeProjection(62))
	if err != nil {
		t.Fatalf("256 nodes rejected: %v", err)
	}
	if _, err := ParseSnapshot(nodeBoundary.Bytes()); err != nil {
		t.Fatalf("stored 256-node snapshot rejected: %v", err)
	}
	if _, err := NewSnapshot(makeNodeProjection(63)); !errors.Is(err, ErrSnapshotTooManyNodes) {
		t.Fatalf("257 nodes error = %v", err)
	}

	large := make([]any, MaxSnapshotMembers)
	for i := range large {
		large[i] = strings.Repeat("x", MaxSnapshotStringBytes)
	}
	if _, err := NewSnapshot(map[string]any{"items": large}); !errors.Is(err, ErrSnapshotTooLarge) {
		t.Fatalf("oversized encoded snapshot error = %v", err)
	}
}

func TestSnapshotRejectsUnsupportedInvalidAndNonObjectProjections(t *testing.T) {
	type projectionStruct struct{ Safe string }
	invalidUTF8 := string([]byte{0xff})
	pointedValue := "value"
	tests := map[string]any{
		"nil root":            nil,
		"scalar root":         "value",
		"array root":          []any{"value"},
		"struct":              map[string]any{"value": projectionStruct{Safe: "yes"}},
		"bytes":               map[string]any{"value": []byte("binary")},
		"function":            map[string]any{"value": func() {}},
		"pointer":             map[string]any{"value": &pointedValue},
		"secret pointer":      map[string]any{"password": &pointedValue},
		"non-string map":      map[string]any{"value": map[int]string{1: "one"}},
		"NaN":                 map[string]any{"value": math.NaN()},
		"positive infinity":   map[string]any{"value": math.Inf(1)},
		"huge JSON number":    map[string]any{"value": json.Number("1e999")},
		"invalid UTF-8 value": map[string]any{"value": invalidUTF8},
		"invalid UTF-8 key":   map[string]any{invalidUTF8: "value"},
	}
	for name, projection := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewSnapshot(projection); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
			}
		})
	}
}

func TestSnapshotRejectsCyclicPointerWithoutHanging(t *testing.T) {
	const helperEnvironment = "GUARDIAN_AUDIT_CYCLIC_POINTER_HELPER"
	if os.Getenv(helperEnvironment) == "1" {
		var cyclic any
		cyclic = &cyclic
		_, err := NewSnapshot(map[string]any{"cycle": cyclic})
		if !errors.Is(err, ErrUnsupportedSnapshotValue) {
			t.Fatalf("cyclic pointer error = %v, want ErrUnsupportedSnapshotValue", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSnapshotRejectsCyclicPointerWithoutHanging$", "-test.count=1")
	command.Env = append(os.Environ(), helperEnvironment+"=1")
	output, err := command.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("cyclic pointer validation did not terminate before timeout; output: %s", output)
	}
	if err != nil {
		t.Fatalf("cyclic pointer helper failed: %v; output: %s", err, output)
	}
}

func TestSnapshotBoundsSelfReferentialContainersByDepth(t *testing.T) {
	cyclicMap := map[string]any{}
	cyclicMap["self"] = cyclicMap
	if _, err := NewSnapshot(cyclicMap); !errors.Is(err, ErrSnapshotTooDeep) {
		t.Fatalf("cyclic map error = %v, want ErrSnapshotTooDeep", err)
	}

	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice
	if _, err := NewSnapshot(map[string]any{"slice": cyclicSlice}); !errors.Is(err, ErrSnapshotTooDeep) {
		t.Fatalf("cyclic slice error = %v, want ErrSnapshotTooDeep", err)
	}
}

func TestParseSnapshotRejectsInvalidOrUnredactedStoredData(t *testing.T) {
	valid := `{"schema":"guardian.audit.snapshot.v1","data":{"password":"[REDACTED]","safe":true}}`
	if _, err := ParseSnapshot([]byte(valid)); err != nil {
		t.Fatalf("valid stored snapshot rejected: %v", err)
	}
	tests := map[string][]byte{
		"unredacted top-level secret": []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"password":"plaintext"}}`),
		"unredacted nested secret":    []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"nested":{"access_token":"plaintext"}}}`),
		"wrong schema":                []byte(`{"schema":"guardian.audit.snapshot.v2","data":{}}`),
		"missing data":                []byte(`{"schema":"guardian.audit.snapshot.v1"}`),
		"extra envelope field":        []byte(`{"schema":"guardian.audit.snapshot.v1","data":{},"extra":true}`),
		"duplicate envelope key":      []byte(`{"schema":"guardian.audit.snapshot.v1","schema":"guardian.audit.snapshot.v1","data":{}}`),
		"duplicate nested key":        []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"safe":1,"safe":2}}`),
		"scalar data":                 []byte(`{"schema":"guardian.audit.snapshot.v1","data":true}`),
		"array data":                  []byte(`{"schema":"guardian.audit.snapshot.v1","data":[]}`),
		"trailing JSON":               []byte(`{"schema":"guardian.audit.snapshot.v1","data":{}}{}`),
		"invalid UTF-8":               append([]byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"`), append([]byte{0xff}, []byte(`"}}`)...)...),
		"oversized input":             []byte(strings.Repeat("x", MaxSnapshotBytes+1)),
	}
	for name, encoded := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseSnapshot(encoded)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("error = %v, want ErrInvalidSnapshot", err)
			}
			if strings.Contains(name, "unredacted") && !errors.Is(err, ErrUnredactedSnapshotSecret) {
				t.Fatalf("error = %v, want ErrUnredactedSnapshotSecret", err)
			}
		})
	}
}

func TestParseSnapshotBoundsHostileExternalDataDuringDecode(t *testing.T) {
	const hostileDepth = 1000
	deepData := strings.Repeat(`{"child":`, hostileDepth) + `{"password":"plaintext"}` + strings.Repeat(`}`, hostileDepth)
	deepEnvelope := []byte(`{"schema":"guardian.audit.snapshot.v1","data":` + deepData + `}`)
	if len(deepEnvelope) > MaxSnapshotBytes {
		t.Fatalf("deep regression fixture is %d bytes; must remain inside the encoded limit", len(deepEnvelope))
	}
	if _, err := ParseSnapshot(deepEnvelope); !errors.Is(err, ErrSnapshotTooDeep) {
		t.Fatalf("deep external snapshot error = %v, want ErrSnapshotTooDeep", err)
	}

	members := make(map[string]any, MaxSnapshotMembers+1)
	for i := 0; i <= MaxSnapshotMembers; i++ {
		members[fmt.Sprintf("member-%02d", i)] = true
	}
	memberEnvelope, err := json.Marshal(map[string]any{"schema": SnapshotSchema, "data": members})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSnapshot(memberEnvelope); !errors.Is(err, ErrSnapshotTooManyMembers) {
		t.Fatalf("external member overflow error = %v, want ErrSnapshotTooManyMembers", err)
	}

	lengths := []int{63, 63, 62, 63}
	arrays := make([]any, len(lengths))
	for i, length := range lengths {
		values := make([]any, length)
		for j := range values {
			values[j] = true
		}
		arrays[i] = values
	}
	nodeEnvelope, err := json.Marshal(map[string]any{
		"schema": SnapshotSchema,
		"data":   map[string]any{"items": arrays},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSnapshot(nodeEnvelope); !errors.Is(err, ErrSnapshotTooManyNodes) {
		t.Fatalf("external node overflow error = %v, want ErrSnapshotTooManyNodes", err)
	}
}

func TestParseSnapshotRejectsLoneSurrogateEscapes(t *testing.T) {
	invalid := map[string][]byte{
		"lone high":        []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\ud800"}}`),
		"lone low":         []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\udc00"}}`),
		"wrong low pair":   []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\ud800\u0041"}}`),
		"surrogate in key": []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"\ud800":"value"}}`),
	}
	for name, encoded := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSnapshot(encoded); !errors.Is(err, ErrUnsupportedSnapshotValue) {
				t.Fatalf("error = %v, want ErrUnsupportedSnapshotValue", err)
			}
		})
	}

	valid := map[string][]byte{
		"surrogate pair":             []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\ud83d\ude00"}}`),
		"escaped replacement rune":   []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\ufffd"}}`),
		"literal replacement rune":   []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"�"}}`),
		"literal surrogate spelling": []byte(`{"schema":"guardian.audit.snapshot.v1","data":{"value":"\\ud800"}}`),
	}
	for name, encoded := range valid {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSnapshot(encoded); err != nil {
				t.Fatalf("valid Unicode snapshot rejected: %v", err)
			}
		})
	}
}

func TestSnapshotJSONRoundTripAndDefensiveBytes(t *testing.T) {
	snapshot, err := NewSnapshot(map[string]any{"safe": "value"})
	if err != nil {
		t.Fatal(err)
	}
	copyBytes := snapshot.Bytes()
	copyBytes[0] = '['
	if snapshot.Bytes()[0] != '{' {
		t.Fatal("Bytes exposed mutable internal state")
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if string(decoded.Bytes()) != string(snapshot.Bytes()) {
		t.Fatalf("round trip mismatch\n got: %s\nwant: %s", decoded.Bytes(), snapshot.Bytes())
	}
	if _, err := json.Marshal(Snapshot{}); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("zero snapshot marshal error = %v", err)
	}
}
