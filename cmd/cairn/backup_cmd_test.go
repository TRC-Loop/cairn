// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TRC-Loop/cairn/internal/backup"
	_ "modernc.org/sqlite"
)

func mustOpenSQLite(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return db
}

func newSilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func seedTestDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "live.db")
	cmd := []string{
		"CREATE TABLE goose_db_version(version_id INTEGER PRIMARY KEY);",
		"INSERT INTO goose_db_version VALUES(1);",
		"CREATE TABLE foo(id INTEGER);",
		"INSERT INTO foo VALUES(99);",
	}
	openAndExec(t, path, strings.Join(cmd, ""))
	return path
}

func openAndExec(t *testing.T, path, sql string) {
	t.Helper()
	dsn := "file:" + path + "?_pragma=foreign_keys(1)"
	db := mustOpenSQLite(t, dsn)
	defer db.Close()
	if _, err := db.Exec(sql); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestBackupCmdMissingDB(t *testing.T) {
	t.Setenv("CAIRN_DB_PATH", filepath.Join(t.TempDir(), "no-such.db"))
	t.Setenv("CAIRN_ENCRYPTION_KEY", "x-pad-32-bytes-pad-pad-pad-pad-x")
	err := runBackupCmd(newSilentLogger(), []string{"-o", filepath.Join(t.TempDir(), "out.db")})
	if err == nil || !strings.Contains(err.Error(), "database not found") {
		t.Fatalf("want missing-db error, got %v", err)
	}
}

func TestBackupCmdPlainWithoutForce(t *testing.T) {
	dbPath := seedTestDB(t)
	t.Setenv("CAIRN_DB_PATH", dbPath)
	t.Setenv("CAIRN_ENCRYPTION_KEY", "x-pad-32-bytes-pad-pad-pad-pad-x")
	err := runBackupCmd(newSilentLogger(), []string{"--plain", "-o", filepath.Join(t.TempDir(), "out.tar.gz")})
	if err == nil || !strings.Contains(err.Error(), "without --force") {
		t.Fatalf("want refusal, got %v", err)
	}
}

func TestBackupCmdConflictPlainAndPassphrase(t *testing.T) {
	dbPath := seedTestDB(t)
	t.Setenv("CAIRN_DB_PATH", dbPath)
	t.Setenv("CAIRN_ENCRYPTION_KEY", "x-pad-32-bytes-pad-pad-pad-pad-x")
	err := runBackupCmd(newSilentLogger(), []string{"--plain", "--passphrase", "--force"})
	if err == nil || !strings.Contains(err.Error(), "either --passphrase or --plain") {
		t.Fatalf("want conflict error, got %v", err)
	}
}

func TestBackupCmdDBOnlyHappyPath(t *testing.T) {
	dbPath := seedTestDB(t)
	t.Setenv("CAIRN_DB_PATH", dbPath)
	t.Setenv("CAIRN_ENCRYPTION_KEY", "x-pad-32-bytes-pad-pad-pad-pad-x")
	out := filepath.Join(t.TempDir(), "out.db")
	if err := runBackupCmd(newSilentLogger(), []string{"-o", out}); err != nil {
		t.Fatalf("backup: %v", err)
	}
	st, err := os.Stat(out)
	if err != nil || st.Size() == 0 {
		t.Fatalf("output missing or empty: %v", err)
	}
}

func TestRestoreCmdRefusesWALPresent(t *testing.T) {
	dbPath := seedTestDB(t)
	if err := os.WriteFile(dbPath+"-wal", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dbPath + "-wal")
	src := seedTestDB(t)
	t.Setenv("CAIRN_DB_PATH", "/dev/null")
	err := runRestoreCmd(newSilentLogger(), []string{"-i", src, "--db", dbPath, "--force"})
	if err == nil || !strings.Contains(err.Error(), "WAL file exists") {
		t.Fatalf("want WAL-detection error, got %v", err)
	}
}

func TestRestoreCmdCorruptIntegrity(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.db")
	// Fake SQLite header but garbage body — integrity_check should fail.
	hdr := []byte("SQLite format 3\x00")
	body := make([]byte, 8192)
	if err := os.WriteFile(bad, append(hdr, body...), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "live.db")
	t.Setenv("CAIRN_DB_PATH", "/dev/null")
	err := runRestoreCmd(newSilentLogger(), []string{"-i", bad, "--db", dst, "--force"})
	if err == nil || !strings.Contains(err.Error(), "integrity_check") {
		t.Fatalf("want integrity_check failure, got %v", err)
	}
}

func TestRestoreCmdWrongPassphraseLeavesLiveAlone(t *testing.T) {
	dbPath := seedTestDB(t)
	t.Setenv("CAIRN_DB_PATH", dbPath)
	t.Setenv("CAIRN_ENCRYPTION_KEY", "x-pad-32-bytes-pad-pad-pad-pad-x")
	bundlePath := filepath.Join(t.TempDir(), "b.cbackup")
	{
		db := mustOpenSQLite(t, "file:"+dbPath+"?_pragma=foreign_keys(1)")
		svc := backup.NewService(db, dbPath, "x-pad-32-bytes-pad-pad-pad-pad-x", "test", newSilentLogger())
		f, _ := os.Create(bundlePath)
		if err := svc.CreateBundle(context.Background(), f, "the-right-passphrase"); err != nil {
			t.Fatalf("create bundle: %v", err)
		}
		f.Close()
		db.Close()
	}
	bundle := bundlePath

	target := filepath.Join(t.TempDir(), "live.db")
	openAndExec(t, target, "CREATE TABLE marker(id INTEGER); INSERT INTO marker VALUES(1);")
	originalBytes, _ := os.ReadFile(target)

	err := runRestoreCmd(newSilentLogger(), []string{"-i", bundle, "--db", target, "--force", "--passphrase", "wrong-passphrase-12chars"})
	if err == nil || !strings.Contains(err.Error(), "decryption failed") {
		t.Fatalf("want decryption failure, got %v", err)
	}
	after, _ := os.ReadFile(target)
	if string(originalBytes) != string(after) {
		t.Fatal("live DB modified after wrong-passphrase restore")
	}
}
