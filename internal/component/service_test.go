// SPDX-License-Identifier: AGPL-3.0-or-later
package component

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

	"github.com/TRC-Loop/cairn/internal/check"
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

func seedCheck(t *testing.T, q *store.Queries, name string, componentID int64, lastStatus check.Status) store.Check {
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
		t.Fatalf("create check %s: %v", name, err)
	}
	if err := q.UpdateCheckStatus(context.Background(), store.UpdateCheckStatusParams{
		LastStatus:           string(lastStatus),
		LastLatencyMs:        sql.NullInt64{},
		LastCheckedAt:        sql.NullTime{},
		ConsecutiveFailures:  0,
		ConsecutiveSuccesses: 0,
		ID:                   c.ID,
	}); err != nil {
		t.Fatalf("set status %s: %v", name, err)
	}
	return c
}

func TestAggregateStatus(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	comp, err := svc.Create(ctx, CreateInput{Name: "API", DisplayOrder: 0})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}

	t.Run("no checks returns unknown", func(t *testing.T) {
		status, err := svc.AggregateStatus(ctx, comp.ID)
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		if status != check.StatusUnknown {
			t.Fatalf("expected unknown, got %s", status)
		}
	})

	t.Run("all up returns up", func(t *testing.T) {
		c1 := seedCheck(t, q, "allup_a", comp.ID, check.StatusUp)
		c2 := seedCheck(t, q, "allup_b", comp.ID, check.StatusUp)
		defer func() {
			_ = q.DeleteCheck(ctx, c1.ID)
			_ = q.DeleteCheck(ctx, c2.ID)
		}()
		status, err := svc.AggregateStatus(ctx, comp.ID)
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		if status != check.StatusUp {
			t.Fatalf("expected up, got %s", status)
		}
	})

	t.Run("degraded wins over up", func(t *testing.T) {
		c1 := seedCheck(t, q, "deg_a", comp.ID, check.StatusUp)
		c2 := seedCheck(t, q, "deg_b", comp.ID, check.StatusDegraded)
		defer func() {
			_ = q.DeleteCheck(ctx, c1.ID)
			_ = q.DeleteCheck(ctx, c2.ID)
		}()
		status, err := svc.AggregateStatus(ctx, comp.ID)
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		if status != check.StatusDegraded {
			t.Fatalf("expected degraded, got %s", status)
		}
	})

	t.Run("down wins over degraded", func(t *testing.T) {
		c1 := seedCheck(t, q, "down_a", comp.ID, check.StatusUp)
		c2 := seedCheck(t, q, "down_b", comp.ID, check.StatusDegraded)
		c3 := seedCheck(t, q, "down_c", comp.ID, check.StatusDown)
		defer func() {
			_ = q.DeleteCheck(ctx, c1.ID)
			_ = q.DeleteCheck(ctx, c2.ID)
			_ = q.DeleteCheck(ctx, c3.ID)
		}()
		status, err := svc.AggregateStatus(ctx, comp.ID)
		if err != nil {
			t.Fatalf("aggregate: %v", err)
		}
		if status != check.StatusDown {
			t.Fatalf("expected down, got %s", status)
		}
	})
}
