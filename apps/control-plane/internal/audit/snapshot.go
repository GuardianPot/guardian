package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	SnapshotSchema         = "guardian.audit.snapshot.v1"
	MaxSnapshotBytes       = 16 * 1024
	MaxSnapshotDepth       = 6
	MaxSnapshotNodes       = 256
	MaxSnapshotMembers     = 64
	MaxSnapshotKeyBytes    = 64
	MaxSnapshotStringBytes = 512
	RedactedSnapshotValue  = "[REDACTED]"
)

var (
	ErrInvalidSnapshot          = errors.New("invalid audit snapshot")
	ErrSnapshotTooLarge         = errors.New("audit snapshot exceeds encoded limit")
	ErrSnapshotTooDeep          = errors.New("audit snapshot exceeds depth limit")
	ErrSnapshotTooManyNodes     = errors.New("audit snapshot exceeds node limit")
	ErrSnapshotTooManyMembers   = errors.New("audit snapshot exceeds member limit")
	ErrSnapshotKeyTooLong       = errors.New("audit snapshot key exceeds byte limit")
	ErrSnapshotStringTooLong    = errors.New("audit snapshot string exceeds byte limit")
	ErrUnsupportedSnapshotValue = errors.New("unsupported audit snapshot value")
	ErrUnredactedSnapshotSecret = errors.New("audit snapshot contains an unredacted secret")
)

// Snapshot is an immutable, validated guardian.audit.snapshot.v1 envelope.
// Its representation is private so invalid or unredacted snapshots cannot be
// assembled without passing NewSnapshot or ParseSnapshot.
type Snapshot struct {
	encoded []byte
}

type snapshotEnvelope struct {
	Schema string `json:"schema"`
	Data   any    `json:"data"`
}

var knownSecretKeys = map[string]struct{}{
	"password":             {},
	"passwords":            {},
	"passwordhash":         {},
	"currentpassword":      {},
	"newpassword":          {},
	"oldpassword":          {},
	"passwordconfirmation": {},
	"token":                {},
	"sessiontoken":         {},
	"bootstraptoken":       {},
	"enrollmenttoken":      {},
	"recoverytoken":        {},
	"accesstoken":          {},
	"refreshtoken":         {},
	"recoverycode":         {},
	"recoverycodes":        {},
	"totpseed":             {},
	"totpsecret":           {},
	"totpcode":             {},
	"mfacode":              {},
	"mfasecret":            {},
	"csrf":                 {},
	"csrftoken":            {},
	"authorization":        {},
	"authorizationheader":  {},
	"cookie":               {},
	"cookies":              {},
	"sessioncookie":        {},
	"privatekey":           {},
	"deviceprivatekey":     {},
	"caprivatekey":         {},
	"tlsprivatekey":        {},
	"signingprivatekey":    {},
	"clientprivatekey":     {},
	"serverprivatekey":     {},
	"secret":               {},
	"apikey":               {},
	"clientsecret":         {},
	"apisecret":            {},
}

var safeSecretReferenceKeys = map[string]struct{}{
	"passwordid":                   {},
	"passwordpolicy":               {},
	"passwordpolicyid":             {},
	"passwordpolicyversion":        {},
	"tokenid":                      {},
	"tokenids":                     {},
	"sessiontokenid":               {},
	"bootstraptokenid":             {},
	"enrollmenttokenid":            {},
	"recoverytokenid":              {},
	"accesstokenid":                {},
	"refreshtokenid":               {},
	"privatekeyid":                 {},
	"privatekeyfingerprint":        {},
	"deviceprivatekeyid":           {},
	"deviceprivatekeyfingerprint":  {},
	"caprivatekeyid":               {},
	"caprivatekeyfingerprint":      {},
	"tlsprivatekeyid":              {},
	"tlsprivatekeyfingerprint":     {},
	"signingprivatekeyid":          {},
	"signingprivatekeyfingerprint": {},
	"clientprivatekeyid":           {},
	"clientprivatekeyfingerprint":  {},
	"serverprivatekeyid":           {},
	"serverprivatekeyfingerprint":  {},
}

var secretKeyStems = []string{
	"deviceprivatekey",
	"signingprivatekey",
	"clientprivatekey",
	"serverprivatekey",
	"caprivatekey",
	"tlsprivatekey",
	"privatekey",
	"enrollmenttoken",
	"bootstraptoken",
	"sessiontoken",
	"bearertoken",
	"authtoken",
	"idtoken",
	"recoverytoken",
	"accesstoken",
	"refreshtoken",
	"csrftoken",
	"recoverycode",
	"totpsecret",
	"totpseed",
	"totpcode",
	"authorizationheader",
	"sessioncookie",
	"clientsecret",
	"apisecret",
	"apikey",
	"mfasecret",
	"mfacode",
	"csrf",
	"passwords",
	"password",
	"secretkey",
	"authorization",
	"cookie",
	"secret",
	"token",
}

var secretValueSuffixes = map[string]struct{}{
	"":            {},
	"value":       {},
	"values":      {},
	"bytes":       {},
	"material":    {},
	"pem":         {},
	"der":         {},
	"raw":         {},
	"plaintext":   {},
	"ciphertext":  {},
	"hash":        {},
	"digest":      {},
	"data":        {},
	"content":     {},
	"hashvalue":   {},
	"digestvalue": {},
	"hashbytes":   {},
	"digestbytes": {},
	"pemvalue":    {},
	"pembytes":    {},
	"dervalue":    {},
	"derbytes":    {},
	"encrypted":   {},
	"encoded":     {},
	"base64":      {},
	"pkcs8":       {},
}

var safeReferenceSuffixes = map[string]struct{}{
	"id":            {},
	"ids":           {},
	"fingerprint":   {},
	"fingerprints":  {},
	"policy":        {},
	"policyid":      {},
	"policyversion": {},
}

var secretValuePrefixes = []string{
	"hashed",
	"current",
	"new",
	"old",
	"raw",
	"plain",
	"plaintext",
	"encoded",
	"encrypted",
}

// NewSnapshot validates a JSON-like safe projection, recursively redacts
// known secret keys, wraps it in the versioned envelope, and enforces the
// exact encoded-size limit. Non-nil pointers, structs, byte slices, custom
// encodings, and other non-JSON projection types are intentionally unsupported.
func NewSnapshot(projection any) (Snapshot, error) {
	nodes := 0
	safe, err := sanitizeSnapshotValue(reflect.ValueOf(projection), 1, &nodes)
	if err != nil {
		return Snapshot{}, err
	}
	if _, ok := safe.(map[string]any); !ok {
		return Snapshot{}, snapshotError(ErrUnsupportedSnapshotValue, "data must be an object")
	}
	encoded, err := json.Marshal(snapshotEnvelope{Schema: SnapshotSchema, Data: safe})
	if err != nil {
		return Snapshot{}, snapshotError(ErrUnsupportedSnapshotValue, "encode projection")
	}
	if len(encoded) > MaxSnapshotBytes {
		return Snapshot{}, snapshotError(ErrSnapshotTooLarge, "encoded snapshot is %d bytes; maximum is %d", len(encoded), MaxSnapshotBytes)
	}
	return Snapshot{encoded: encoded}, nil
}

// ParseSnapshot validates an externally supplied envelope and rebuilds its
// canonical, redacted representation. Unknown/duplicate fields and trailing
// JSON values are rejected.
func ParseSnapshot(encoded []byte) (Snapshot, error) {
	if len(encoded) == 0 {
		return Snapshot{}, snapshotError(nil, "snapshot is empty")
	}
	if len(encoded) > MaxSnapshotBytes {
		return Snapshot{}, snapshotError(ErrSnapshotTooLarge, "encoded snapshot is %d bytes; maximum is %d", len(encoded), MaxSnapshotBytes)
	}
	if !utf8.Valid(encoded) {
		return Snapshot{}, snapshotError(nil, "snapshot JSON is not valid UTF-8")
	}
	if err := validateJSONUnicodeEscapes(encoded); err != nil {
		return Snapshot{}, snapshotError(ErrUnsupportedSnapshotValue, "snapshot JSON contains an invalid Unicode escape: %v", err)
	}
	envelope, err := decodeSnapshotEnvelope(encoded)
	if err != nil {
		if errors.Is(err, ErrInvalidSnapshot) {
			return Snapshot{}, err
		}
		return Snapshot{}, snapshotError(nil, "decode envelope: %v", err)
	}
	if len(envelope) != 2 {
		return Snapshot{}, snapshotError(nil, "envelope must contain exactly schema and data")
	}
	schema, ok := envelope["schema"].(string)
	if !ok || schema != SnapshotSchema {
		return Snapshot{}, snapshotError(nil, "unsupported or missing schema")
	}
	projection, ok := envelope["data"]
	if !ok {
		return Snapshot{}, snapshotError(nil, "data is required")
	}
	return NewSnapshot(projection)
}

// Bytes returns a defensive copy of the canonical envelope.
func (s Snapshot) Bytes() []byte {
	return bytes.Clone(s.encoded)
}

// IsZero reports whether s has never passed a snapshot constructor/parser.
func (s Snapshot) IsZero() bool {
	return len(s.encoded) == 0
}

// Validate verifies the private representation, including redaction and all
// current limits.
func (s Snapshot) Validate() error {
	if s.IsZero() {
		return snapshotError(nil, "snapshot is zero")
	}
	_, err := ParseSnapshot(s.encoded)
	return err
}

// MarshalJSON emits the complete versioned envelope.
func (s Snapshot) MarshalJSON() ([]byte, error) {
	if s.IsZero() {
		return nil, snapshotError(nil, "cannot marshal a zero snapshot")
	}
	return s.Bytes(), nil
}

// UnmarshalJSON accepts only a complete, valid versioned envelope.
func (s *Snapshot) UnmarshalJSON(encoded []byte) error {
	if s == nil {
		return snapshotError(nil, "snapshot destination is nil")
	}
	parsed, err := ParseSnapshot(encoded)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

// IsKnownSecretKey applies exact matching plus narrow secret-stem/value-suffix
// rules after case/separator normalization. Explicit reference exceptions keep
// token IDs, private-key fingerprints, and password-policy metadata visible.
//
// This classifier is defense in depth, not a substitute for caller-supplied
// safe projections. Generic or misleading keys such as "payload" or
// "credential_blob" cannot be reliably classified and remain caller risk.
func IsKnownSecretKey(key string) bool {
	normalized := normalizeSecretKey(key)
	if _, safe := safeSecretReferenceKeys[normalized]; safe {
		return false
	}
	if matchesSecretStemWithSuffixes(normalized, safeReferenceSuffixes) {
		return false
	}
	if _, known := knownSecretKeys[normalized]; known {
		return true
	}
	if matchesSecretStemAndSuffix(normalized) {
		return true
	}
	for _, prefix := range secretValuePrefixes {
		if strings.HasPrefix(normalized, prefix) && matchesSecretStemAndSuffix(strings.TrimPrefix(normalized, prefix)) {
			return true
		}
	}
	return false
}

func matchesSecretStemAndSuffix(normalized string) bool {
	return matchesSecretStemWithSuffixes(normalized, secretValueSuffixes)
}

func matchesSecretStemWithSuffixes(normalized string, suffixes map[string]struct{}) bool {
	for _, stem := range secretKeyStems {
		if !strings.HasPrefix(normalized, stem) {
			continue
		}
		if _, allowed := suffixes[strings.TrimPrefix(normalized, stem)]; allowed {
			return true
		}
	}
	return false
}

func normalizeSecretKey(key string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(key) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func validateJSONUnicodeEscapes(encoded []byte) error {
	inString := false
	for index := 0; index < len(encoded); index++ {
		switch encoded[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString || index+1 >= len(encoded) {
				continue
			}
			if encoded[index+1] != 'u' {
				index++
				continue
			}
			codeUnit, ok := parseJSONCodeUnit(encoded, index+2)
			if !ok {
				return errors.New("malformed \\u escape")
			}
			switch {
			case codeUnit >= 0xd800 && codeUnit <= 0xdbff:
				if index+11 >= len(encoded) || encoded[index+6] != '\\' || encoded[index+7] != 'u' {
					return errors.New("high surrogate is not followed by a low surrogate")
				}
				low, ok := parseJSONCodeUnit(encoded, index+8)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return errors.New("high surrogate is not followed by a low surrogate")
				}
				index += 11
			case codeUnit >= 0xdc00 && codeUnit <= 0xdfff:
				return errors.New("low surrogate has no leading high surrogate")
			default:
				index += 5
			}
		}
	}
	return nil
}

func parseJSONCodeUnit(encoded []byte, start int) (uint16, bool) {
	if start+4 > len(encoded) {
		return 0, false
	}
	var value uint16
	for _, character := range encoded[start : start+4] {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value |= uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value |= uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value |= uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func sanitizeSnapshotValue(value reflect.Value, depth int, nodes *int) (any, error) {
	if value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			return countSnapshotNode(nil, depth, nodes)
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return countSnapshotNode(nil, depth, nodes)
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return countSnapshotNode(nil, depth, nodes)
		}
		return nil, snapshotError(ErrUnsupportedSnapshotValue, "non-nil pointers are not safe projections")
	}

	if depth > MaxSnapshotDepth {
		return nil, snapshotError(ErrSnapshotTooDeep, "depth %d exceeds %d", depth, MaxSnapshotDepth)
	}
	*nodes++
	if *nodes > MaxSnapshotNodes {
		return nil, snapshotError(ErrSnapshotTooManyNodes, "node count exceeds %d", MaxSnapshotNodes)
	}

	if value.CanInterface() {
		if number, ok := value.Interface().(json.Number); ok {
			if !json.Valid([]byte(number.String())) {
				return nil, snapshotError(ErrUnsupportedSnapshotValue, "invalid JSON number")
			}
			parsed, err := strconv.ParseFloat(number.String(), 64)
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return nil, snapshotError(ErrUnsupportedSnapshotValue, "JSON number is outside the supported finite range")
			}
			return number, nil
		}
	}

	switch value.Kind() {
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.String:
		text := value.String()
		if !utf8.ValidString(text) {
			return nil, snapshotError(ErrUnsupportedSnapshotValue, "string is not valid UTF-8")
		}
		if len(text) > MaxSnapshotStringBytes {
			return nil, snapshotError(ErrSnapshotStringTooLong, "string is %d bytes; maximum is %d", len(text), MaxSnapshotStringBytes)
		}
		return text, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint(), nil
	case reflect.Float32, reflect.Float64:
		number := value.Float()
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, snapshotError(ErrUnsupportedSnapshotValue, "non-finite number")
		}
		return number, nil
	case reflect.Map:
		if value.IsNil() {
			return nil, nil
		}
		if value.Type().Key().Kind() != reflect.String {
			return nil, snapshotError(ErrUnsupportedSnapshotValue, "object keys must be strings")
		}
		if value.Len() > MaxSnapshotMembers {
			return nil, snapshotError(ErrSnapshotTooManyMembers, "object has %d members; maximum is %d", value.Len(), MaxSnapshotMembers)
		}
		object := make(map[string]any, value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			key := iterator.Key().String()
			if !utf8.ValidString(key) {
				return nil, snapshotError(ErrUnsupportedSnapshotValue, "object key is not valid UTF-8")
			}
			if len(key) > MaxSnapshotKeyBytes {
				return nil, snapshotError(ErrSnapshotKeyTooLong, "key is %d bytes; maximum is %d", len(key), MaxSnapshotKeyBytes)
			}
			projected, err := sanitizeSnapshotValue(iterator.Value(), depth+1, nodes)
			if err != nil {
				return nil, err
			}
			if IsKnownSecretKey(key) {
				projected = RedactedSnapshotValue
			}
			object[key] = projected
		}
		return object, nil
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return nil, nil
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return nil, snapshotError(ErrUnsupportedSnapshotValue, "byte arrays are not safe projections")
		}
		if value.Len() > MaxSnapshotMembers {
			return nil, snapshotError(ErrSnapshotTooManyMembers, "array has %d members; maximum is %d", value.Len(), MaxSnapshotMembers)
		}
		array := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			projected, err := sanitizeSnapshotValue(value.Index(i), depth+1, nodes)
			if err != nil {
				return nil, err
			}
			array[i] = projected
		}
		return array, nil
	default:
		return nil, snapshotError(ErrUnsupportedSnapshotValue, "kind %s", value.Kind())
	}
}

func countSnapshotNode(value any, depth int, nodes *int) (any, error) {
	if depth > MaxSnapshotDepth {
		return nil, snapshotError(ErrSnapshotTooDeep, "depth %d exceeds %d", depth, MaxSnapshotDepth)
	}
	*nodes++
	if *nodes > MaxSnapshotNodes {
		return nil, snapshotError(ErrSnapshotTooManyNodes, "node count exceeds %d", MaxSnapshotNodes)
	}
	return value, nil
}

func snapshotError(cause error, format string, args ...any) error {
	detail := fmt.Errorf(format, args...)
	if cause == nil {
		return fmt.Errorf("%w: %v", ErrInvalidSnapshot, detail)
	}
	return fmt.Errorf("%w: %w: %v", ErrInvalidSnapshot, cause, detail)
}

func decodeSnapshotEnvelope(encoded []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, errors.New("envelope must be an object")
	}

	envelope := make(map[string]any, 2)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("envelope key is not a string")
		}
		if _, duplicate := envelope[key]; duplicate {
			return nil, fmt.Errorf("duplicate envelope key %q", key)
		}
		switch key {
		case "schema":
			value, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			if _, nested := value.(json.Delim); nested {
				return nil, errors.New("schema must be a string")
			}
			envelope[key] = value
		case "data":
			nodes := 0
			value, err := decodeBoundedSnapshotJSONValue(decoder, 1, &nodes)
			if err != nil {
				return nil, err
			}
			envelope[key] = value
		default:
			return nil, fmt.Errorf("unknown envelope key %q", key)
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return nil, errors.New("unterminated envelope")
	}
	if err := ensureJSONTokenEOF(decoder); err != nil {
		return nil, err
	}
	return envelope, nil
}

func decodeBoundedSnapshotJSONValue(decoder *json.Decoder, depth int, nodes *int) (any, error) {
	if depth > MaxSnapshotDepth {
		return nil, snapshotError(ErrSnapshotTooDeep, "depth %d exceeds %d", depth, MaxSnapshotDepth)
	}
	*nodes++
	if *nodes > MaxSnapshotNodes {
		return nil, snapshotError(ErrSnapshotTooManyNodes, "node count exceeds %d", MaxSnapshotNodes)
	}

	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		switch value := token.(type) {
		case nil, bool:
			return token, nil
		case string:
			if len(value) > MaxSnapshotStringBytes {
				return nil, snapshotError(ErrSnapshotStringTooLong, "string is %d bytes; maximum is %d", len(value), MaxSnapshotStringBytes)
			}
			return value, nil
		case json.Number:
			parsed, err := strconv.ParseFloat(value.String(), 64)
			if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
				return nil, snapshotError(ErrUnsupportedSnapshotValue, "JSON number is outside the supported finite range")
			}
			return value, nil
		default:
			return nil, fmt.Errorf("unsupported JSON token %T", token)
		}
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		members := 0
		for decoder.More() {
			members++
			if members > MaxSnapshotMembers {
				return nil, snapshotError(ErrSnapshotTooManyMembers, "object has more than %d members", MaxSnapshotMembers)
			}
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("object key is not a string")
			}
			if len(key) > MaxSnapshotKeyBytes {
				return nil, snapshotError(ErrSnapshotKeyTooLong, "key is %d bytes; maximum is %d", len(key), MaxSnapshotKeyBytes)
			}
			if _, duplicate := object[key]; duplicate {
				return nil, fmt.Errorf("duplicate object key %q", key)
			}
			value, err := decodeBoundedSnapshotJSONValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			if IsKnownSecretKey(key) {
				redacted, ok := value.(string)
				if !ok || redacted != RedactedSnapshotValue {
					return nil, snapshotError(ErrUnredactedSnapshotSecret, "key %q is not redacted", key)
				}
			}
			object[key] = value
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return nil, errors.New("unterminated object")
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for decoder.More() {
			if len(array) >= MaxSnapshotMembers {
				return nil, snapshotError(ErrSnapshotTooManyMembers, "array has more than %d members", MaxSnapshotMembers)
			}
			value, err := decodeBoundedSnapshotJSONValue(decoder, depth+1, nodes)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return nil, errors.New("unterminated array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected delimiter %q", delimiter)
	}
}

func ensureJSONTokenEOF(decoder *json.Decoder) error {
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON value")
	}
	return err
}
