// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

type Status string

const (
	StatusInvestigating Status = "investigating"
	StatusIdentified    Status = "identified"
	StatusMonitoring    Status = "monitoring"
	StatusResolved      Status = "resolved"
)

type Severity string

const (
	SeverityMinor    Severity = "minor"
	SeverityMajor    Severity = "major"
	SeverityCritical Severity = "critical"
)

func (s Status) Valid() bool {
	switch s {
	case StatusInvestigating, StatusIdentified, StatusMonitoring, StatusResolved:
		return true
	}
	return false
}

func (s Severity) Valid() bool {
	switch s {
	case SeverityMinor, SeverityMajor, SeverityCritical:
		return true
	}
	return false
}

// ValidTransition returns true if the given status transition is permitted.
// Same-state is treated as a no-op (an update can be posted without changing
// status). Regression (monitoring → investigating) and reopen (resolved →
// investigating) are both allowed; callers should log a warning when reopening.
func ValidTransition(from, to Status) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusInvestigating:
		return to == StatusIdentified || to == StatusMonitoring || to == StatusResolved
	case StatusIdentified:
		return to == StatusMonitoring || to == StatusResolved
	case StatusMonitoring:
		return to == StatusResolved || to == StatusInvestigating
	case StatusResolved:
		return to == StatusInvestigating
	}
	return false
}
