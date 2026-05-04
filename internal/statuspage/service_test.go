// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

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

func TestSetDefaultPromotesAndDemotes(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	first, err := svc.Create(ctx, CreateInput{Slug: "first", Title: "First", IsDefault: true})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	if !first.IsDefault {
		t.Fatal("expected first to be default on creation")
	}

	second, err := svc.Create(ctx, CreateInput{Slug: "second", Title: "Second"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	if err := svc.SetDefault(ctx, second.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}

	refreshedFirst, err := q.GetStatusPage(ctx, first.ID)
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if refreshedFirst.IsDefault {
		t.Fatal("expected first to no longer be default")
	}
	refreshedSecond, err := q.GetStatusPage(ctx, second.ID)
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if !refreshedSecond.IsDefault {
		t.Fatal("expected second to be default")
	}
}

func TestVerifyPassword(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	page, err := svc.Create(ctx, CreateInput{Slug: "private", Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if ok, err := svc.VerifyPassword(ctx, "private", "anything"); err != nil || !ok {
		t.Fatalf("unprotected page must verify any password: ok=%v err=%v", ok, err)
	}

	if err := svc.SetPassword(ctx, page.ID, "s3cret"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	ok, err := svc.VerifyPassword(ctx, "private", "s3cret")
	if err != nil {
		t.Fatalf("verify correct: %v", err)
	}
	if !ok {
		t.Fatal("expected correct password to verify")
	}

	bad, err := svc.VerifyPassword(ctx, "private", "nope")
	if err != nil {
		t.Fatalf("verify wrong: %v", err)
	}
	if bad {
		t.Fatal("wrong password verified as correct")
	}

	if err := svc.SetPassword(ctx, page.ID, ""); err != nil {
		t.Fatalf("clear password: %v", err)
	}
	if ok, err := svc.VerifyPassword(ctx, "private", "whatever"); err != nil || !ok {
		t.Fatalf("cleared password must verify any password: ok=%v err=%v", ok, err)
	}
}

func TestComponentOrdering(t *testing.T) {
	svc, q := newTestService(t)
	ctx := context.Background()

	page, err := svc.Create(ctx, CreateInput{Slug: "ord", Title: "Ord"})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	compIDs := make([]int64, 0, 3)
	for _, n := range []string{"a", "b", "c"} {
		c, err := q.CreateComponent(ctx, store.CreateComponentParams{Name: n, DisplayOrder: 0})
		if err != nil {
			t.Fatalf("create component %s: %v", n, err)
		}
		compIDs = append(compIDs, c.ID)
	}

	for i, cid := range compIDs {
		if err := svc.AddComponent(ctx, page.ID, cid, int64(i)); err != nil {
			t.Fatalf("add component: %v", err)
		}
	}

	reordered := []int64{compIDs[2], compIDs[0], compIDs[1]}
	if err := svc.ReorderComponents(ctx, page.ID, reordered); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	got, err := svc.ListComponents(ctx, page.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	gotIDs := []int64{got[0].ID, got[1].ID, got[2].ID}
	for i, want := range reordered {
		if gotIDs[i] != want {
			t.Fatalf("position %d: expected %d, got %d (all=%v)", i, want, gotIDs[i], gotIDs)
		}
	}
}
