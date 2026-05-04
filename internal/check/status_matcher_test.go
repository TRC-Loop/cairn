// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import "testing"

func TestStatusMatcherSingle(t *testing.T) {
	m, err := ParseStatusMatcher("200")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !m.Matches(200) || m.Matches(201) {
		t.Fatalf("single match wrong: 200=%v 201=%v", m.Matches(200), m.Matches(201))
	}
}

func TestStatusMatcherRange(t *testing.T) {
	m, err := ParseStatusMatcher("200-299")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, code := range []int{200, 250, 299} {
		if !m.Matches(code) {
			t.Errorf("expected %d to match", code)
		}
	}
	for _, code := range []int{199, 300} {
		if m.Matches(code) {
			t.Errorf("expected %d not to match", code)
		}
	}
}

func TestStatusMatcherList(t *testing.T) {
	m, err := ParseStatusMatcher("200,201,204")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, code := range []int{200, 201, 204} {
		if !m.Matches(code) {
			t.Errorf("expected %d to match", code)
		}
	}
	for _, code := range []int{202, 203} {
		if m.Matches(code) {
			t.Errorf("expected %d not to match", code)
		}
	}
}

func TestStatusMatcherMixed(t *testing.T) {
	m, err := ParseStatusMatcher("200-204,301,302")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, code := range []int{200, 202, 204, 301, 302} {
		if !m.Matches(code) {
			t.Errorf("expected %d to match", code)
		}
	}
	for _, code := range []int{205, 300, 303} {
		if m.Matches(code) {
			t.Errorf("expected %d not to match", code)
		}
	}
}

func TestStatusMatcherEmptyDefaults(t *testing.T) {
	m, err := ParseStatusMatcher("")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, code := range []int{200, 250, 299} {
		if !m.Matches(code) {
			t.Errorf("expected %d to match default", code)
		}
	}
	if m.Matches(300) {
		t.Errorf("expected 300 not to match default")
	}
}

func TestStatusMatcherMalformed(t *testing.T) {
	bad := []string{"abc", "200-", "-300", "200-100", "200,,300", "99", "600", "-1"}
	for _, s := range bad {
		if _, err := ParseStatusMatcher(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}
