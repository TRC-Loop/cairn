// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"database/sql"
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
			t.Fatalf("read: %v", err)
		}
		idx := strings.Index(string(raw), "-- +goose Down")
		body := string(raw)
		if idx >= 0 {
			body = body[:idx]
		}
		body = gooseDirective.ReplaceAllString(body, "")
		if _, err := db.Exec(body); err != nil {
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

func createTestUser(t *testing.T, q *store.Queries, username, password, role string) store.User {
	t.Helper()
	hash, err := Hash(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := q.CreateUser(context.Background(), store.CreateUserParams{
		Username:     username,
		Email:        username + "@example.com",
		DisplayName:  username,
		PasswordHash: hash,
		Role:         role,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}
