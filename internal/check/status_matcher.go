// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"fmt"
	"strconv"
	"strings"
)

// StatusMatcher matches HTTP status codes against a spec like "200-299,301,302".
type StatusMatcher struct {
	spec   string
	ranges []statusRange
}

type statusRange struct {
	min, max int
}

const defaultStatusMatcherSpec = "200-299"

// ParseStatusMatcher parses a spec string. Empty input defaults to "200-299".
// Codes must lie in the inclusive range 100-599. Returns an error for
// malformed input (non-numeric pieces, negative numbers, reversed ranges,
// empty pieces between commas, or codes outside 100-599).
func ParseStatusMatcher(spec string) (*StatusMatcher, error) {
	original := spec
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = defaultStatusMatcherSpec
		original = defaultStatusMatcherSpec
	}

	pieces := strings.Split(spec, ",")
	ranges := make([]statusRange, 0, len(pieces))
	for _, raw := range pieces {
		piece := strings.TrimSpace(raw)
		if piece == "" {
			return nil, fmt.Errorf("empty piece in status spec %q", original)
		}
		if dash := strings.IndexByte(piece, '-'); dash != -1 {
			lo := strings.TrimSpace(piece[:dash])
			hi := strings.TrimSpace(piece[dash+1:])
			if lo == "" || hi == "" {
				return nil, fmt.Errorf("malformed range %q", piece)
			}
			a, err := parseStatusInt(lo)
			if err != nil {
				return nil, err
			}
			b, err := parseStatusInt(hi)
			if err != nil {
				return nil, err
			}
			if a > b {
				return nil, fmt.Errorf("reversed range %d-%d", a, b)
			}
			ranges = append(ranges, statusRange{min: a, max: b})
		} else {
			v, err := parseStatusInt(piece)
			if err != nil {
				return nil, err
			}
			ranges = append(ranges, statusRange{min: v, max: v})
		}
	}

	return &StatusMatcher{spec: original, ranges: ranges}, nil
}

func parseStatusInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid status code %q", s)
	}
	if v < 100 || v > 599 {
		return 0, fmt.Errorf("status code %d outside 100-599", v)
	}
	return v, nil
}

// Matches reports whether code falls within the matcher's set.
func (m *StatusMatcher) Matches(code int) bool {
	if m == nil {
		return false
	}
	for _, r := range m.ranges {
		if code >= r.min && code <= r.max {
			return true
		}
	}
	return false
}

// String returns the original spec string.
func (m *StatusMatcher) String() string {
	if m == nil {
		return ""
	}
	return m.spec
}
