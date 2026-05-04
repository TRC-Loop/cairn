// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected PHC format: %s", h)
	}
	parts := strings.Split(h, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 PHC segments, got %d in %q", len(parts), h)
	}

	ok, err := Verify("correct horse battery staple", h)
	if err != nil {
		t.Fatalf("verify correct: %v", err)
	}
	if !ok {
		t.Fatal("correct password did not verify")
	}

	bad, err := Verify("wrong password", h)
	if err != nil {
		t.Fatalf("verify wrong: %v", err)
	}
	if bad {
		t.Fatal("wrong password verified as correct")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	if _, err := Verify("x", "not-a-hash"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}

func TestHashDifferentEachCall(t *testing.T) {
	a, err := Hash("pw")
	if err != nil {
		t.Fatalf("hash a: %v", err)
	}
	b, err := Hash("pw")
	if err != nil {
		t.Fatalf("hash b: %v", err)
	}
	if a == b {
		t.Fatal("two hashes of the same password collided; salt is not random")
	}
}
