// SPDX-License-Identifier: AGPL-3.0-or-later
package migrations_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/TRC-Loop/cairn/migrations"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestAutoMigrationAppliesOnFreshDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("up: %v", err)
	}

	mustExist := []string{"status_pages", "components", "checks", "incidents", "maintenance_windows"}
	for _, name := range mustExist {
		var got string
		row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, name)
		if err := row.Scan(&got); err != nil {
			t.Errorf("expected table %q, got err %v", name, err)
		}
	}

	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("second up (should no-op): %v", err)
	}
}

func TestMigrationsEmbedFSLoadsAllSQLFiles(t *testing.T) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var count int
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".sql" {
			count++
		}
	}
	if count == 0 {
		t.Fatal("embedded FS has no .sql files")
	}
}
