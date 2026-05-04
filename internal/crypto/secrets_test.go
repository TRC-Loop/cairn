// SPDX-License-Identifier: AGPL-3.0-or-later
package crypto

import (
	"strings"
	"testing"
)

const testKey = "this-is-a-32-byte-test-master-key-xxxxx"

func TestSecretBoxRoundTrip(t *testing.T) {
	sb, err := NewSecretBox(testKey)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	for _, plain := range []string{"", "x", "smtp_pa$$word!", strings.Repeat("a", 4096)} {
		ct, err := sb.EncryptString(plain)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		if ct == plain {
			t.Fatalf("ciphertext equals plaintext")
		}
		got, err := sb.DecryptString(ct)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if got != plain {
			t.Fatalf("round-trip mismatch: %q vs %q", got, plain)
		}
	}
}

func TestSecretBoxNonceIsRandom(t *testing.T) {
	sb, _ := NewSecretBox(testKey)
	a, _ := sb.EncryptString("same-input")
	b, _ := sb.EncryptString("same-input")
	if a == b {
		t.Fatal("expected distinct ciphertexts due to random nonce")
	}
}

func TestSecretBoxKeyTooShort(t *testing.T) {
	if _, err := NewSecretBox("short"); err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestSecretBoxRejectsTamperedCiphertext(t *testing.T) {
	sb, _ := NewSecretBox(testKey)
	ct, _ := sb.EncryptString("payload")
	tampered := ct[:len(ct)-2] + "AA"
	if _, err := sb.DecryptString(tampered); err == nil {
		t.Fatal("expected decryption to fail on tampered ciphertext")
	}
}

func TestSecretBoxRejectsWrongKey(t *testing.T) {
	a, _ := NewSecretBox(testKey)
	b, _ := NewSecretBox("a-different-32-byte-master-key-here-zzz")
	ct, _ := a.EncryptString("payload")
	if _, err := b.DecryptString(ct); err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}
