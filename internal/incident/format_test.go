// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

import (
	"testing"
	"time"
)

func TestFormatID(t *testing.T) {
	started := time.Date(2026, 4, 27, 9, 5, 30, 0, time.UTC)
	cases := []struct {
		template string
		id       int64
		want     string
	}{
		{"#INC-{id}", 42, "#INC-42"},
		{"", 7, "#INC-7"},
		{"{year}-{id}", 99, "2026-99"},
		{"{year}{month}{day}-{id}", 1, "20260427-1"},
		{"INC-{datetime}-{id}", 5, "INC-20260427T090530-5"},
		{"#{unknown}-{id}", 3, "#{unknown}-3"},
	}
	for _, c := range cases {
		got := FormatID(c.template, c.id, started)
		if got != c.want {
			t.Errorf("FormatID(%q, %d) = %q, want %q", c.template, c.id, got, c.want)
		}
	}
}
