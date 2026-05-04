// SPDX-License-Identifier: AGPL-3.0-or-later
package maintenance

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
			t.Fatalf("read %s: %v", f, err)
		}
		up := extractUp(string(raw))
		if _, err := db.Exec(up); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
}

func extractUp(content string) string {
	idx := strings.Index(content, "-- +goose Down")
	if idx == -1 {
		idx = len(content)
	}
	return gooseDirective.ReplaceAllString(content[:idx], "")
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

func newTestService(t *testing.T) (*Service, *store.Queries) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(db, q, logger), q
}

func TestStateSchedulerTransitions(t *testing.T) {
	_, q := newTestService(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sched := NewStateScheduler(q, logger)

	past := time.Now().UTC().Add(-5 * time.Minute)
	future := time.Now().UTC().Add(1 * time.Hour)
	scheduled, err := q.CreateMaintenanceWindow(ctx, store.CreateMaintenanceWindowParams{
		Title:    "due to start",
		StartsAt: past,
		EndsAt:   future,
		State:    StateScheduled,
	})
	if err != nil {
		t.Fatalf("create scheduled: %v", err)
	}

	longAgo := time.Now().UTC().Add(-2 * time.Hour)
	almostGone := time.Now().UTC().Add(-1 * time.Minute)
	ending, err := q.CreateMaintenanceWindow(ctx, store.CreateMaintenanceWindowParams{
		Title:    "due to complete",
		StartsAt: longAgo,
		EndsAt:   almostGone,
		State:    StateInProgress,
	})
	if err != nil {
		t.Fatalf("create in-progress: %v", err)
	}

	sched.RunOnce(ctx)

	got1, err := q.GetMaintenanceWindow(ctx, scheduled.ID)
	if err != nil {
		t.Fatalf("get scheduled: %v", err)
	}
	if got1.State != StateInProgress {
		t.Fatalf("expected in_progress, got %s", got1.State)
	}

	got2, err := q.GetMaintenanceWindow(ctx, ending.ID)
	if err != nil {
		t.Fatalf("get ending: %v", err)
	}
	if got2.State != StateCompleted {
		t.Fatalf("expected completed, got %s", got2.State)
	}
}

func TestIsCheckUnderMaintenance(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	comp, err := q.CreateComponent(ctx, store.CreateComponentParams{Name: "API", DisplayOrder: 0})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "api-http",
		Type:              "http",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  3,
		RecoveryThreshold: 1,
		ConfigJson:        `{}`,
		ComponentID:       sql.NullInt64{Int64: comp.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}

	under, err := svc.IsCheckUnderMaintenance(ctx, c.ID)
	if err != nil {
		t.Fatalf("is under: %v", err)
	}
	if under {
		t.Fatal("expected not under maintenance")
	}

	w, err := q.CreateMaintenanceWindow(ctx, store.CreateMaintenanceWindowParams{
		Title:    "planned",
		StartsAt: time.Now().UTC().Add(-1 * time.Minute),
		EndsAt:   time.Now().UTC().Add(1 * time.Hour),
		State:    StateInProgress,
	})
	if err != nil {
		t.Fatalf("create window: %v", err)
	}
	if err := q.LinkComponent(ctx, store.LinkComponentParams{MaintenanceID: w.ID, ComponentID: comp.ID}); err != nil {
		t.Fatalf("link: %v", err)
	}

	under, err = svc.IsCheckUnderMaintenance(ctx, c.ID)
	if err != nil {
		t.Fatalf("is under 2: %v", err)
	}
	if !under {
		t.Fatal("expected check to be under maintenance")
	}
}
