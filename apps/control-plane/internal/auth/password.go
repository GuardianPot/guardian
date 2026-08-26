package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/text/unicode/norm"
)

const (
	MinimumPasswordRunes = 12
	MaximumPasswordRunes = 128
	MaximumPasswordBytes = 1024
)

var (
	ErrInvalidPassword = errors.New("password does not meet the approved policy")
	ErrInvalidHash     = errors.New("password hash is invalid")
)

// Argon2Params is stored with every PHC record so work factors can increase
// without making existing credentials unverifiable.
type Argon2Params struct {
	MemoryKiB   uint32
	Passes      uint32
	Lanes       uint8
	SaltBytes   uint32
	OutputBytes uint32
}

var DefaultArgon2Params = Argon2Params{
	MemoryKiB:   64 * 1024,
	Passes:      3,
	Lanes:       4,
	SaltBytes:   16,
	OutputBytes: 32,
}

func (p Argon2Params) Validate() error {
	if p.MemoryKiB < DefaultArgon2Params.MemoryKiB || p.MemoryKiB > 1024*1024 ||
		p.Passes < DefaultArgon2Params.Passes || p.Passes > 16 ||
		p.Lanes < DefaultArgon2Params.Lanes || p.Lanes > 32 ||
		p.SaltBytes < DefaultArgon2Params.SaltBytes || p.SaltBytes > 64 ||
		p.OutputBytes < DefaultArgon2Params.OutputBytes || p.OutputBytes > 64 {
		return ErrInvalidHash
	}
	return nil
}

// NormalizeNewPassword applies the approved NFC, size, and local blocklist
// policy. Password text is never trimmed or case-folded before hashing.
func NormalizeNewPassword(password, username string) (string, error) {
	normalized, err := normalizePassword(password)
	if err != nil {
		return "", err
	}
	comparison := strings.ToLower(normalized)
	blocked := map[string]struct{}{
		"123456789012": {}, "administrator": {}, "changeme123!": {},
		"guardian123!": {}, "guardianpot": {}, "letmein12345": {},
		"password123!": {}, "qwertyuiop12": {}, "welcome12345": {},
	}
	if _, found := blocked[comparison]; found {
		return "", ErrInvalidPassword
	}
	user := strings.ToLower(strings.TrimSpace(username))
	if user != "" && (comparison == user || comparison == user+"123456" || comparison == "guardian"+user) {
		return "", ErrInvalidPassword
	}
	return normalized, nil
}

func normalizePassword(password string) (string, error) {
	if !utf8.ValidString(password) || len(password) > MaximumPasswordBytes {
		return "", ErrInvalidPassword
	}
	normalized := norm.NFC.String(password)
	count := utf8.RuneCountInString(normalized)
	if count < MinimumPasswordRunes || count > MaximumPasswordRunes || len(normalized) > MaximumPasswordBytes {
		return "", ErrInvalidPassword
	}
	return normalized, nil
}

func HashPassword(password string, params Argon2Params) (string, error) {
	if err := params.Validate(); err != nil {
		return "", err
	}
	normalized, err := normalizePassword(password)
	if err != nil {
		return "", err
	}
	salt := make([]byte, params.SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(normalized), salt, params.Passes, params.MemoryKiB, params.Lanes, params.OutputBytes)
	defer clear(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, params.MemoryKiB, params.Passes, params.Lanes,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

// VerifyPassword rejects malformed or weaker PHC records before doing any
// attacker-controlled allocation. The caller uses a fixed dummy PHC for
// unknown accounts to keep the external login path equivalent.
func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, expected, err := parsePHC(encoded)
	if err != nil {
		return false, err
	}
	normalized, err := normalizePassword(password)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(normalized), salt, params.Passes, params.MemoryKiB, params.Lanes, uint32(len(expected)))
	defer clear(actual)
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parsePHC(encoded string) (Argon2Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return Argon2Params{}, nil, nil, ErrInvalidHash
	}
	settings := strings.Split(parts[3], ",")
	if len(settings) != 3 {
		return Argon2Params{}, nil, nil, ErrInvalidHash
	}
	memory, errM := parseSetting(settings[0], "m", 32)
	passes, errT := parseSetting(settings[1], "t", 32)
	lanes, errP := parseSetting(settings[2], "p", 8)
	if errM != nil || errT != nil || errP != nil {
		return Argon2Params{}, nil, nil, ErrInvalidHash
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil {
		return Argon2Params{}, nil, nil, ErrInvalidHash
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil {
		return Argon2Params{}, nil, nil, ErrInvalidHash
	}
	params := Argon2Params{
		MemoryKiB: uint32(memory), Passes: uint32(passes), Lanes: uint8(lanes),
		SaltBytes: uint32(len(salt)), OutputBytes: uint32(len(hash)),
	}
	if err := params.Validate(); err != nil {
		return Argon2Params{}, nil, nil, err
	}
	return params, salt, hash, nil
}

func parseSetting(value, key string, bits int) (uint64, error) {
	prefix := key + "="
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, ErrInvalidHash
	}
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bits)
	if err != nil {
		return 0, ErrInvalidHash
	}
	return parsed, nil
}
