// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"regexp"
	"testing"
)

var urlSafe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestGenerateTokenLengthAndShape(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		tok, err := GenerateToken()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(tok) != 43 {
			t.Fatalf("expected 43 chars, got %d (%q)", len(tok), tok)
		}
		if !urlSafe.MatchString(tok) {
			t.Fatalf("token not URL-safe: %q", tok)
		}
		if _, ok := seen[tok]; ok {
			t.Fatalf("duplicate token generated: %q", tok)
		}
		seen[tok] = struct{}{}
	}
}
