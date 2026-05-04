// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

func createComponent(t *testing.T, q *store.Queries, name string) store.Component {
	t.Helper()
	c, err := q.CreateComponent(context.Background(), store.CreateComponentParams{
		Name: name,
	})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	return c
}

func createCheckFor(t *testing.T, q *store.Queries, componentID int64, name string) store.Check {
	t.Helper()
	c, err := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name:              name,
		Type:              "http",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  3,
		RecoveryThreshold: 1,
		ConfigJson:        `{}`,
		ComponentID:       sql.NullInt64{Int64: componentID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	return c
}

func upsertDaily(t *testing.T, q *store.Queries, checkID int64, day time.Time, up, degraded, down int64) {
	t.Helper()
	if err := q.UpsertCheckResultDaily(context.Background(), store.UpsertCheckResultDailyParams{
		CheckID:       checkID,
		DayBucket:     day,
		TotalCount:    up + degraded + down,
		UpCount:       up,
		DegradedCount: degraded,
		DownCount:     down,
	}); err != nil {
		t.Fatalf("upsert daily: %v", err)
	}
}

func TestHistoryForSevenDaysWithMaintenanceOverlay(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	comp := createComponent(t, q, "api")
	c := createCheckFor(t, q, comp.ID, "http-api")

	// Build 5 aggregate rows over the last 7 days. Days 6,5,4,3,2 have data.
	today := startOfDayUTC(time.Now().UTC())
	days := 7
	first := today.AddDate(0, 0, -(days - 1))

	upsertDaily(t, q, c.ID, first.AddDate(0, 0, 0), 100, 0, 0) // up
	upsertDaily(t, q, c.ID, first.AddDate(0, 0, 1), 90, 10, 0) // degraded
	upsertDaily(t, q, c.ID, first.AddDate(0, 0, 2), 50, 0, 50) // down
	upsertDaily(t, q, c.ID, first.AddDate(0, 0, 3), 100, 0, 0) // up
	upsertDaily(t, q, c.ID, first.AddDate(0, 0, 4), 100, 0, 0) // up
	// days 5 and 6 (today) are missing → nodata

	// Maintenance window spanning day 4 → overlay should override that "up" day.
	maintStart := first.AddDate(0, 0, 4)
	maintEnd := maintStart.Add(6 * time.Hour)
	mw, err := q.CreateMaintenanceWindow(ctx, store.CreateMaintenanceWindowParams{
		Title:    "planned reboot",
		StartsAt: maintStart,
		EndsAt:   maintEnd,
		State:    "completed",
	})
	if err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	if err := q.LinkComponent(ctx, store.LinkComponentParams{MaintenanceID: mw.ID, ComponentID: comp.ID}); err != nil {
		t.Fatalf("link maintenance component: %v", err)
	}

	hist, err := svc.HistoryFor(ctx, comp.ID, days)
	if err != nil {
		t.Fatalf("HistoryFor: %v", err)
	}
	if len(hist) != days {
		t.Fatalf("expected %d history entries, got %d", days, len(hist))
	}

	want := []string{
		dayStatusUp,
		dayStatusDegraded,
		dayStatusDown,
		dayStatusUp,
		dayStatusMaintenance, // overlaid
		dayStatusNoData,
		dayStatusNoData,
	}
	for i, exp := range want {
		if hist[i].Status != exp {
			t.Errorf("day %d: expected %q, got %q (date=%s)", i, exp, hist[i].Status, hist[i].Date.Format("2006-01-02"))
		}
	}
	if !hist[0].Date.Equal(first) {
		t.Errorf("expected first date %s, got %s", first.Format("2006-01-02"), hist[0].Date.Format("2006-01-02"))
	}
	if !hist[len(hist)-1].Date.Equal(today) {
		t.Errorf("expected last date %s, got %s", today.Format("2006-01-02"), hist[len(hist)-1].Date.Format("2006-01-02"))
	}
}

func TestHistoryForMultiCheckWorstStatus(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	comp := createComponent(t, q, "multi")
	c1 := createCheckFor(t, q, comp.ID, "c1")
	c2 := createCheckFor(t, q, comp.ID, "c2")

	today := startOfDayUTC(time.Now().UTC())
	first := today.AddDate(0, 0, -2)

	// day 0: c1 up, c2 down → worst = down
	upsertDaily(t, q, c1.ID, first, 100, 0, 0)
	upsertDaily(t, q, c2.ID, first, 0, 0, 10)
	// day 1: c1 degraded, c2 up → worst = degraded
	upsertDaily(t, q, c1.ID, first.AddDate(0, 0, 1), 90, 10, 0)
	upsertDaily(t, q, c2.ID, first.AddDate(0, 0, 1), 100, 0, 0)

	hist, err := svc.HistoryFor(ctx, comp.ID, 3)
	if err != nil {
		t.Fatalf("HistoryFor: %v", err)
	}
	if hist[0].Status != dayStatusDown {
		t.Errorf("day 0 should be down (worst of up+down), got %s", hist[0].Status)
	}
	if hist[1].Status != dayStatusDegraded {
		t.Errorf("day 1 should be degraded, got %s", hist[1].Status)
	}
	if hist[2].Status != dayStatusNoData {
		t.Errorf("day 2 should be nodata, got %s", hist[2].Status)
	}
}
