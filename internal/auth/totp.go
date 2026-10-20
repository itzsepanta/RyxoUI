package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP parameters.
const (
	// TOTPStep is the time step in seconds.
	TOTPStep = 30
	// TOTPDigits is the length of a generated code.
	TOTPDigits = 6
	// TOTPSkew is the number of steps accepted on either side.
	TOTPSkew = 1
)

// GenerateTOTPSecret returns a new base32 secret.
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: read totp secret: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

// VerifyTOTP reports whether code is valid for secret at the given time.
func VerifyTOTP(secret, code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	if secret == "" || len(code) != TOTPDigits {
		return false
	}

	counter := at.UTC().Unix() / TOTPStep
	for offset := -TOTPSkew; offset <= TOTPSkew; offset++ {
		expected := hotp(secret, counter+int64(offset))
		if subtle.ConstantTimeCompare([]byte(expected), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// CodeForTime returns the one time password of a secret at a given moment.
// It exists so the tests and the tools can produce a valid code without
// duplicating the algorithm.
func CodeForTime(secret string, at time.Time) string {
	return hotp(secret, at.UTC().Unix()/TOTPStep)
}

// hotp computes the HMAC based one time password of a counter.
func hotp(secret string, counter int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	var buf [8]byte
	counter64 := uint64(counter)
	for i := 7; i >= 0; i-- {
		buf[i] = byte(counter64)
		counter64 >>= 8
	}

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := (int64(sum[offset]&0x7f) << 24) |
		(int64(sum[offset+1]) << 16) |
		(int64(sum[offset+2]) << 8) |
		int64(sum[offset+3])

	mod := int64(1)
	for i := 0; i < TOTPDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", TOTPDigits, value%mod)
}

// TOTPURI builds the otpauth URI used by authenticator applications.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprintf("%d", TOTPDigits))
	query.Set("period", fmt.Sprintf("%d", TOTPStep))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// GenerateRecoveryCodes returns a list of single use recovery codes.
func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		count = 8
	}
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		code, err := RandomToken(8)
		if err != nil {
			return nil, err
		}
		codes = append(codes, strings.ToUpper(code))
	}
	return codes, nil
}
