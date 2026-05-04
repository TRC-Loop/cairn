// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	TOTPIssuer    = "Cairn"
	TOTPDigits    = otp.DigitsSix
	TOTPPeriod    = 30
	TOTPAlgorithm = otp.AlgorithmSHA1

	RecoveryCodeGroups   = 4
	RecoveryCodeGroupLen = 3
	DefaultRecoveryCount = 10
)

// recoveryAlphabet excludes 0/O/1/I/L to reduce read/write confusion.
const recoveryAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// GenerateTOTPSecret returns a new random TOTP secret (base32-encoded) and a QR-code-ready otpauth URL.
func GenerateTOTPSecret(accountName string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: accountName,
		Period:      TOTPPeriod,
		Digits:      TOTPDigits,
		Algorithm:   TOTPAlgorithm,
	})
	if err != nil {
		return "", "", fmt.Errorf("totp generate: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// VerifyTOTP checks code against secret with ±1 period skew tolerance.
func VerifyTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" || secret == "" {
		return false
	}
	ok, err := totp.ValidateCustom(code, secret, nowFunc(), totp.ValidateOpts{
		Period:    TOTPPeriod,
		Skew:      1,
		Digits:    TOTPDigits,
		Algorithm: TOTPAlgorithm,
	})
	if err != nil {
		return false
	}
	return ok
}

// nowFunc is overridable in tests.
var nowFunc = func() time.Time { return time.Now().UTC() }

// GenerateRecoveryCodes produces n random codes plus their argon2id hashes.
// Plaintext is shown to the user once; only hashes are stored.
func GenerateRecoveryCodes(n int) (plaintext, hashes []string, err error) {
	if n <= 0 {
		return nil, nil, fmt.Errorf("recovery code count must be positive")
	}
	plaintext = make([]string, 0, n)
	hashes = make([]string, 0, n)
	seen := make(map[string]struct{}, n)
	for len(plaintext) < n {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, nil, err
		}
		if _, dup := seen[code]; dup {
			continue
		}
		seen[code] = struct{}{}
		h, err := Hash(code)
		if err != nil {
			return nil, nil, fmt.Errorf("hash recovery code: %w", err)
		}
		plaintext = append(plaintext, code)
		hashes = append(hashes, h)
	}
	return plaintext, hashes, nil
}

func generateRecoveryCode() (string, error) {
	total := RecoveryCodeGroups * RecoveryCodeGroupLen
	buf := make([]byte, total)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	out := make([]byte, 0, total+RecoveryCodeGroups-1)
	for i := 0; i < total; i++ {
		if i > 0 && i%RecoveryCodeGroupLen == 0 {
			out = append(out, '-')
		}
		out = append(out, recoveryAlphabet[int(buf[i])%len(recoveryAlphabet)])
	}
	return string(out), nil
}

// VerifyRecoveryCode checks code against an argon2id hash. Constant-time.
func VerifyRecoveryCode(code, hash string) bool {
	code = NormalizeRecoveryCode(code)
	if code == "" || hash == "" {
		return false
	}
	ok, err := Verify(code, hash)
	if err != nil {
		return false
	}
	return ok
}

// NormalizeRecoveryCode uppercases and strips whitespace; preserves dashes.
func NormalizeRecoveryCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.ToUpper(code)
	var b strings.Builder
	b.Grow(len(code))
	for _, r := range code {
		if r == ' ' || r == '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LooksLikeRecoveryCode returns true if the input has the dash-separated shape;
// false suggests a TOTP digit code.
func LooksLikeRecoveryCode(s string) bool {
	return strings.Contains(s, "-")
}

// constantTimeEqual is a small helper for cases that don't need full hash compare.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
