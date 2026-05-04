// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import "testing"

func TestRedactDSN(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"postgres://user:secret@host:5432/db", "postgres://user:****@host:5432/db"},
		{"postgres://user:secret@host:5432/db?sslmode=disable", "postgres://user:****@host:5432/db?sslmode=disable"},
		{"user:hunter2@tcp(host:3306)/db", "user:****@tcp(host:3306)/db"},
		{"redis://localhost:6379", "redis://localhost:6379"},
		{"no-creds-here", "no-creds-here"},
	}
	for _, tt := range tests {
		got := RedactDSN(tt.in)
		if got != tt.want {
			t.Errorf("RedactDSN(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
