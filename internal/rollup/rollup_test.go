// SPDX-License-Identifier: AGPL-3.0-or-later
package rollup

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
	_ "modernc.org/sqlite"
)

var gooseDirective = regexp.MustCompile(`(?m)^\s*--\s*\+goose.*$`)

func openTestDB(t *testing.T) (*sql.DB, *store.Queries) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	applyMigrations(t, db)
	return db, store.New(db)
}

func applyMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	root := findRepoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		idx := strings.Index(string(raw), "-- +goose Down")
		if idx == -1 {
			idx = len(raw)
		}
		up := gooseDirective.ReplaceAllString(string(raw)[:idx], "")
		if _, err := db.Exec(up); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			t.Fatalf("go.mod not found")
		}
		dir = p
	}
}

func makeCheck(t *testing.T, q *store.Queries) store.Check {
	t.Helper()
	c, err := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name:              "c",
		Type:              "http",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  1,
		RecoveryThreshold: 1,
		ConfigJson:        `{}`,
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	return c
}

func insertRaw(t *testing.T, q *store.Queries, checkID int64, at time.Time, status string, latency *int64) {
	t.Helper()
	var lat sql.NullInt64
	if latency != nil {
		lat = sql.NullInt64{Int64: *latency, Valid: true}
	}
	if err := q.InsertCheckResult(context.Background(), store.InsertCheckResultParams{
		CheckID:   checkID,
		CheckedAt: at,
		Status:    status,
		LatencyMs: lat,
	}); err != nil {
		t.Fatalf("insert raw: %v", err)
	}
}

func TestRollupHourly(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()
	c := makeCheck(t, q)

	hour := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	for i, lat := range []int64{10, 20, 30, 40, 50} {
		l := lat
		insertRaw(t, q, c.ID, hour.Add(time.Duration(i)*time.Minute), "up", &l)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := New(db, q, logger, time.Hour)
	if err := r.RunOnce(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	rows, err := q.GetHourlyInRange(ctx, store.GetHourlyInRangeParams{
		CheckID:      c.ID,
		HourBucket:   hour,
		HourBucket_2: hour,
	})
	if err != nil {
		t.Fatalf("get hourly: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 hourly row, got %d", len(rows))
	}
	h := rows[0]
	if h.TotalCount != 5 || h.UpCount != 5 {
		t.Fatalf("counts wrong: total=%d up=%d", h.TotalCount, h.UpCount)
	}
	if !h.MinLatencyMs.Valid || h.MinLatencyMs.Int64 != 10 {
		t.Fatalf("min wrong: %+v", h.MinLatencyMs)
	}
	if !h.MaxLatencyMs.Valid || h.MaxLatencyMs.Int64 != 50 {
		t.Fatalf("max wrong: %+v", h.MaxLatencyMs)
	}
	if !h.AvgLatencyMs.Valid || h.AvgLatencyMs.Float64 != 30 {
		t.Fatalf("avg wrong: %+v", h.AvgLatencyMs)
	}
}

func TestRollupRetentionDeletesOldRaw(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()
	c := makeCheck(t, q)

	if err := q.UpdateRetentionSettings(ctx, store.UpdateRetentionSettingsParams{
		RawDays:          1,
		HourlyDays:       30,
		DailyDays:        180,
		KeepDailyForever: false,
	}); err != nil {
		t.Fatalf("retention: %v", err)
	}

	old := time.Now().UTC().Add(-3 * 24 * time.Hour)
	recent := time.Now().UTC().Add(-1 * time.Hour)
	l := int64(100)
	insertRaw(t, q, c.ID, old, "up", &l)
	insertRaw(t, q, c.ID, recent, "up", &l)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := New(db, q, logger, time.Hour)
	if err := r.RunOnce(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	rows, err := q.GetRecentResults(ctx, store.GetRecentResultsParams{CheckID: c.ID, Limit: 100})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after retention, got %d", len(rows))
	}
	if rows[0].CheckedAt.Before(recent.Add(-time.Minute)) {
		t.Fatalf("wrong row survived")
	}
}
