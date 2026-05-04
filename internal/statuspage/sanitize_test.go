// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"strings"
	"testing"
)

func TestSanitizeFooter(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		mustHave  []string
		mustNotHv []string
	}{
		{
			name:      "strips script tag",
			in:        `hello <script>alert(1)</script> world`,
			mustHave:  []string{"hello", "world"},
			mustNotHv: []string{"<script", "alert(1)"},
		},
		{
			name:      "strips javascript: href",
			in:        `<a href="javascript:alert(1)">click</a>`,
			mustHave:  []string{"click"},
			mustNotHv: []string{"javascript:", "alert(1)"},
		},
		{
			name:      "allowed tags pass through",
			in:        `<p>Hello <strong>world</strong><br><em>!</em></p>`,
			mustHave:  []string{"<p>", "<strong>", "<br", "<em>"},
			mustNotHv: []string{"<script"},
		},
		{
			name:      "allowed link with https stays",
			in:        `<a href="https://example.com">link</a>`,
			mustHave:  []string{`href="https://example.com"`, `rel=`, `link`},
			mustNotHv: []string{"javascript:"},
		},
		{
			name:      "style attribute stripped",
			in:        `<span style="color: red">x</span>`,
			mustHave:  []string{"<span>", "x"},
			mustNotHv: []string{"style=", "color: red"},
		},
		{
			name:      "onclick attribute stripped",
			in:        `<a href="https://ok" onclick="alert(1)">x</a>`,
			mustHave:  []string{"x"},
			mustNotHv: []string{"onclick", "alert(1)"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(SanitizeFooter(tc.in))
			for _, s := range tc.mustHave {
				if !strings.Contains(got, s) {
					t.Errorf("expected output to contain %q; got %q", s, got)
				}
			}
			for _, s := range tc.mustNotHv {
				if strings.Contains(got, s) {
					t.Errorf("expected output to NOT contain %q; got %q", s, got)
				}
			}
		})
	}
}
