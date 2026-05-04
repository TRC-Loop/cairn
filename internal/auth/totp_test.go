// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func freezeTime(t *testing.T, at time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return at }
	t.Cleanup(func() { nowFunc = prev })
}

func TestGenerateAndVerifyTOTP(t *testing.T) {
	secret, url, err := GenerateTOTPSecret("alice@example.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if secret == "" {
		t.Fatal("empty secret")
	}
	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Fatalf("unexpected url: %s", url)
	}
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if !VerifyTOTP(secret, code) {
		t.Fatal("expected freshly generated code to verify")
	}
	if VerifyTOTP(secret, "000000") {
		t.Fatal("zero code should not verify (unless lucky)")
	}
}

func TestVerifyTOTPSkew(t *testing.T) {
	secret, _, err := GenerateTOTPSecret("alice@example.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	now := time.Now().UTC()
	cases := []struct {
		offset time.Duration
		want   bool
	}{
		{0, true},
		{-30 * time.Second, true},
		{30 * time.Second, true},
		{-90 * time.Second, false},
		{90 * time.Second, false},
	}
	for _, tc := range cases {
		ts := now.Add(tc.offset)
		code, err := totp.GenerateCode(secret, ts)
		if err != nil {
			t.Fatalf("code @ %v: %v", tc.offset, err)
		}
		freezeTime(t, now)
		got := VerifyTOTP(secret, code)
		if got != tc.want {
			t.Errorf("offset=%v want=%v got=%v code=%s", tc.offset, tc.want, got, code)
		}
	}
}

func TestRecoveryCodeFormat(t *testing.T) {
	codes, hashes, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if len(codes) != 10 || len(hashes) != 10 {
		t.Fatalf("counts wrong: %d/%d", len(codes), len(hashes))
	}
	seen := map[string]bool{}
	for i, c := range codes {
		if seen[c] {
			t.Fatalf("duplicate code: %s", c)
		}
		seen[c] = true
		parts := strings.Split(c, "-")
		if len(parts) != RecoveryCodeGroups {
			t.Fatalf("expected %d groups, got %d in %s", RecoveryCodeGroups, len(parts), c)
		}
		for _, p := range parts {
			if len(p) != RecoveryCodeGroupLen {
				t.Fatalf("group length wrong: %s", c)
			}
			for _, r := range p {
				if strings.ContainsRune("01OIL", r) {
					t.Fatalf("ambiguous char in %s", c)
				}
			}
		}
		if !VerifyRecoveryCode(c, hashes[i]) {
			t.Fatalf("code %s did not verify against its hash", c)
		}
	}
}

func TestRecoveryCodeUniqueAcrossManyGenerations(t *testing.T) {
	// Tests the raw code generator (no argon2 hashing) for collision resistance.
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		c, err := generateRecoveryCode()
		if err != nil {
			t.Fatalf("gen: %v", err)
		}
		if seen[c] {
			t.Fatalf("collision after %d generations: %s", i, c)
		}
		seen[c] = true
	}
}

func TestVerifyRecoveryCodeWrong(t *testing.T) {
	_, hashes, err := GenerateRecoveryCodes(1)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if VerifyRecoveryCode("AAA-AAA-AAA-AAA", hashes[0]) {
		t.Fatal("wrong code should not verify")
	}
	if VerifyRecoveryCode("", hashes[0]) {
		t.Fatal("empty code should not verify")
	}
}

func TestNormalizeRecoveryCode(t *testing.T) {
	got := NormalizeRecoveryCode(" 7h2-xk9-pqm-4dt ")
	if got != "7H2-XK9-PQM-4DT" {
		t.Fatalf("got %q", got)
	}
}
