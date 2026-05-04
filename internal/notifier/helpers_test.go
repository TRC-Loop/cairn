// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
	_ "modernc.org/sqlite"
)

const testMasterKey = "test-master-key-32-bytes-or-more!!"

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
	return db, store.New(db)
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

func testSecretBox(t *testing.T) *crypto.SecretBox {
	t.Helper()
	sb, err := crypto.NewSecretBox(testMasterKey)
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	return sb
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
