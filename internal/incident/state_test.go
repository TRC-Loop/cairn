// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

import "testing"

func TestValidTransition(t *testing.T) {
	cases := []struct {
		from, to Status
		want     bool
	}{
		{StatusInvestigating, StatusIdentified, true},
		{StatusInvestigating, StatusMonitoring, true},
		{StatusInvestigating, StatusResolved, true},
		{StatusInvestigating, StatusInvestigating, true},

		{StatusIdentified, StatusMonitoring, true},
		{StatusIdentified, StatusResolved, true},
		{StatusIdentified, StatusInvestigating, false},
		{StatusIdentified, StatusIdentified, true},

		{StatusMonitoring, StatusResolved, true},
		{StatusMonitoring, StatusInvestigating, true},
		{StatusMonitoring, StatusIdentified, false},
		{StatusMonitoring, StatusMonitoring, true},

		{StatusResolved, StatusInvestigating, true},
		{StatusResolved, StatusIdentified, false},
		{StatusResolved, StatusMonitoring, false},
		{StatusResolved, StatusResolved, true},

		{Status("bogus"), StatusResolved, false},
	}

	for _, c := range cases {
		got := ValidTransition(c.from, c.to)
		if got != c.want {
			t.Errorf("ValidTransition(%s,%s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
