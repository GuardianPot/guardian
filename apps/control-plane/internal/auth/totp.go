package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	TOTPSeedBytes = 32
	TOTPStep      = 30 * time.Second
	TOTPDigits    = 6
)

var ErrInvalidTOTP = errors.New("TOTP value is invalid")

func GenerateTOTPSeed() ([]byte, error) {
	seed := make([]byte, TOTPSeedBytes)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generate TOTP seed: %w", err)
	}
	return seed, nil
}

func ProvisioningURI(issuer, account string, seed []byte) (string, error) {
	if issuer == "" || account == "" || len(seed) != TOTPSeedBytes {
		return "", ErrInvalidTOTP
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(seed)
	label := url.PathEscape(issuer + ":" + account)
	values := url.Values{
		"algorithm": {"SHA256"}, "digits": {strconv.Itoa(TOTPDigits)},
		"issuer": {issuer}, "period": {strconv.Itoa(int(TOTPStep.Seconds()))}, "secret": {secret},
	}
	return "otpauth://totp/" + label + "?" + values.Encode(), nil
}

func TOTPCode(seed []byte, at time.Time) (string, error) {
	if len(seed) != TOTPSeedBytes {
		return "", ErrInvalidTOTP
	}
	return codeForCounter(seed, at.UTC().Unix()/int64(TOTPStep.Seconds())), nil
}

// VerifyTOTP accepts one adjacent step in either direction for clock drift and
// returns the matched counter. Callers persist it and require strict increase.
func VerifyTOTP(seed []byte, code string, at time.Time, lastAccepted int64) (int64, bool) {
	if len(seed) != TOTPSeedBytes || len(code) != TOTPDigits || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	current := at.UTC().Unix() / int64(TOTPStep.Seconds())
	matched := int64(-1)
	for _, candidate := range []int64{current - 1, current, current + 1} {
		expected := codeForCounter(seed, candidate)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 && candidate > lastAccepted {
			matched = candidate
		}
	}
	return matched, matched >= 0
}

func codeForCounter(seed []byte, counter int64) string {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, uint64(counter))
	mac := hmac.New(sha256.New, seed)
	_, _ = mac.Write(buffer)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", TOTPDigits, value%1_000_000)
}
