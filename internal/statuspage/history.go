// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

// DayStatus is one rectangle on the 90-day bar.
// Status is one of: "up", "degraded", "down", "maintenance", "nodata".
type DayStatus struct {
	Date   time.Time `json:"date"`
	Status string    `json:"status"`
}

const (
	dayStatusUp          = "up"
	dayStatusDegraded    = "degraded"
	dayStatusDown        = "down"
	dayStatusMaintenance = "maintenance"
	dayStatusNoData      = "nodata"
)

// HistoryFor returns `days` daily status entries for the component, earliest
// to latest. Per-day resolution takes the worst status across the component's
// member checks; days within any maintenance window that touches the
// component get overlaid as "maintenance". Accepts the N+1 shape for v1 —
// one daily-bucket query per check in the component.
func (s *Service) HistoryFor(ctx context.Context, componentID int64, days int) ([]DayStatus, error) {
	if days <= 0 {
		return []DayStatus{}, nil
	}

	today := startOfDayUTC(time.Now().UTC())
	start := today.AddDate(0, 0, -(days - 1))

	checks, err := s.q.ListChecksForComponent(ctx, sql.NullInt64{Int64: componentID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list checks for component %d: %w", componentID, err)
	}

	out := make([]DayStatus, days)
	for i := 0; i < days; i++ {
		out[i] = DayStatus{Date: start.AddDate(0, 0, i), Status: dayStatusNoData}
	}

	for _, c := range checks {
		rows, err := s.q.GetDailyInRange(ctx, store.GetDailyInRangeParams{
			CheckID:     c.ID,
			DayBucket:   start,
			DayBucket_2: today,
		})
		if err != nil {
			return nil, fmt.Errorf("daily for check %d: %w", c.ID, err)
		}
		for _, r := range rows {
			idx := dayIndex(start, r.DayBucket, days)
			if idx < 0 || idx >= days {
				continue
			}
			out[idx].Status = worseStatus(out[idx].Status, dayFromCounts(r))
		}
	}

	if err := s.overlayMaintenance(ctx, componentID, start, today, out); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Service) overlayMaintenance(ctx context.Context, componentID int64, start, today time.Time, out []DayStatus) error {
	const q = `
SELECT mw.starts_at, mw.ends_at
FROM maintenance_affected_components mac
JOIN maintenance_windows mw ON mw.id = mac.maintenance_id
WHERE mac.component_id = ?
  AND mw.state IN ('in_progress','completed','scheduled')
  AND mw.starts_at <= ?
  AND mw.ends_at   >= ?`
	rangeEnd := today.Add(24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, q, componentID, rangeEnd, start)
	if err != nil {
		return fmt.Errorf("maintenance overlay: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var s0, e0 time.Time
		if err := rows.Scan(&s0, &e0); err != nil {
			return fmt.Errorf("scan maintenance overlay: %w", err)
		}
		applyMaintenanceOverlay(out, s0, e0)
	}
	return rows.Err()
}

func applyMaintenanceOverlay(out []DayStatus, startsAt, endsAt time.Time) {
	if len(out) == 0 {
		return
	}
	first := out[0].Date
	last := out[len(out)-1].Date
	s0 := startOfDayUTC(startsAt)
	e0 := startOfDayUTC(endsAt)
	if s0.Before(first) {
		s0 = first
	}
	if e0.After(last) {
		e0 = last
	}
	for d := s0; !d.After(e0); d = d.AddDate(0, 0, 1) {
		idx := dayIndex(first, d, len(out))
		if idx >= 0 && idx < len(out) {
			out[idx].Status = dayStatusMaintenance
		}
	}
}

func dayFromCounts(r store.CheckResultsDaily) string {
	if r.DownCount > 0 {
		return dayStatusDown
	}
	if r.DegradedCount > 0 {
		return dayStatusDegraded
	}
	if r.UpCount > 0 {
		return dayStatusUp
	}
	return dayStatusNoData
}

// worseStatus picks the more alarming of two day statuses.
// Ordering (worst first): down > degraded > maintenance > up > nodata.
// Note: maintenance overlay is applied after, so the precedence here matters
// only when merging multiple checks within the same day.
func worseStatus(a, b string) string {
	rank := map[string]int{
		dayStatusDown:        5,
		dayStatusDegraded:    4,
		dayStatusMaintenance: 3,
		dayStatusUp:          2,
		dayStatusNoData:      1,
		"":                   0,
	}
	if rank[a] >= rank[b] {
		return a
	}
	return b
}

func startOfDayUTC(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func dayIndex(start, day time.Time, max int) int {
	diff := int(startOfDayUTC(day).Sub(start) / (24 * time.Hour))
	if diff < 0 || diff >= max {
		return -1
	}
	return diff
}

// UptimePercent returns a display-ready "99.81%" / "—" string for a slice.
// Maintenance and nodata are excluded from the denominator; up is the only
// contributor to the numerator.
func UptimePercent(history []DayStatus) string {
	var up, counted int
	for _, d := range history {
		switch d.Status {
		case dayStatusUp:
			up++
			counted++
		case dayStatusDegraded, dayStatusDown:
			counted++
		}
	}
	if counted == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", float64(up)/float64(counted)*100)
}
