// SPDX-License-Identifier: AGPL-3.0-or-later
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "live.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE goose_db_version (version_id INTEGER PRIMARY KEY); INSERT INTO goose_db_version(version_id) VALUES(7);`); err != nil {
		t.Fatalf("seed goose: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`); err != nil {
		t.Fatalf("seed table: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := db.Exec(`INSERT INTO widgets(name) VALUES (?)`, fmt.Sprintf("w-%d", i)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return db, path
}

func newSvc(t *testing.T) (*Service, string) {
	db, path := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(db, path, "test-master-key-32bytes-padding!!", "test", logger), path
}

func TestCreateDBOnlyRoundTrip(t *testing.T) {
	svc, _ := newSvc(t)
	var buf bytes.Buffer
	n, err := svc.CreateDBOnly(context.Background(), &buf)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if n == 0 || buf.Len() == 0 {
		t.Fatal("empty backup")
	}
	out := filepath.Join(t.TempDir(), "out.db")
	if err := os.WriteFile(out, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	db, err := sql.Open("sqlite", "file:"+out+"?mode=ro")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 5 {
		t.Fatalf("want 5 rows, got %d", count)
	}
}

func TestCreateBundlePlain(t *testing.T) {
	svc, _ := newSvc(t)
	var buf bytes.Buffer
	if err := svc.CreateBundle(context.Background(), &buf, ""); err != nil {
		t.Fatalf("bundle: %v", err)
	}
	gz, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	found := map[string]bool{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		found[hdr.Name] = true
		_, _ = io.ReadAll(tr)
	}
	for _, name := range []string{tarDBName, tarKeyName, tarManifestName} {
		if !found[name] {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestCreateBundleEncryptedRoundTrip(t *testing.T) {
	svc, _ := newSvc(t)
	var buf bytes.Buffer
	if err := svc.CreateBundle(context.Background(), &buf, "correct-passphrase-12chars"); err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if string(buf.Bytes()[:4]) != BundleMagic {
		t.Fatal("missing magic header")
	}
	if _, err := MaybeDecryptBundle(buf.Bytes(), "wrong-passphrase-12chars"); !errors.Is(err, ErrBadPassphrase) {
		t.Fatalf("want ErrBadPassphrase, got %v", err)
	}
	plain, err := MaybeDecryptBundle(buf.Bytes(), "correct-passphrase-12chars")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	m, dbBytes, key, err := readBundle(plain)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if m.Mode != ModeBundleEncrypted {
		t.Fatalf("mode=%s", m.Mode)
	}
	if m.SchemaVersion != 7 {
		t.Fatalf("schema=%d", m.SchemaVersion)
	}
	if len(dbBytes) == 0 || len(key) == 0 {
		t.Fatal("missing payload")
	}
}

func TestRestoreDBOnly(t *testing.T) {
	svc, livePath := newSvc(t)
	var buf bytes.Buffer
	if _, err := svc.CreateDBOnly(context.Background(), &buf); err != nil {
		t.Fatalf("create: %v", err)
	}
	// Close live DB
	if err := svc.db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := svc.RestoreDBOnly(context.Background(), &buf); err != nil {
		t.Fatalf("restore: %v", err)
	}
	db, err := sql.Open("sqlite", "file:"+livePath+"?mode=ro")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 5 {
		t.Fatalf("want 5, got %d", n)
	}
}

func TestRestoreBundleEncrypted(t *testing.T) {
	svc, livePath := newSvc(t)
	var buf bytes.Buffer
	pass := "passphrase-correct-here"
	if err := svc.CreateBundle(context.Background(), &buf, pass); err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.db.Close()
	m, key, err := svc.RestoreBundle(context.Background(), &buf, pass)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if m == nil || string(key) != "test-master-key-32bytes-padding!!" {
		t.Fatalf("manifest/key mismatch: m=%v key=%q", m, string(key))
	}
	db, _ := sql.Open("sqlite", "file:"+livePath+"?mode=ro")
	defer db.Close()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 5 {
		t.Fatalf("want 5, got %d", n)
	}
}

func TestRestoreCorruptedDBLeavesLiveAlone(t *testing.T) {
	svc, livePath := newSvc(t)
	svc.db.Close()
	originalBytes, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	garbage := bytes.NewReader([]byte("not a sqlite database, just garbage"))
	err = svc.RestoreDBOnly(context.Background(), garbage)
	if !errors.Is(err, ErrIntegrityCheck) {
		t.Fatalf("want ErrIntegrityCheck, got %v", err)
	}
	after, _ := os.ReadFile(livePath)
	if !bytes.Equal(originalBytes, after) {
		t.Fatal("live DB was modified despite integrity failure")
	}
	if _, err := os.Stat(livePath + ".restore.tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file not cleaned up")
	}
}

func TestRestoreBundleRejectsFormatVersion(t *testing.T) {
	svc, livePath := newSvc(t)
	var buf bytes.Buffer
	if err := svc.CreateBundle(context.Background(), &buf, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	plain, _ := MaybeDecryptBundle(buf.Bytes(), "")
	tampered := bumpManifest(t, plain, "999")

	svc.db.Close()
	originalBytes, _ := os.ReadFile(livePath)
	_, _, err := svc.RestoreBundle(context.Background(), bytes.NewReader(tampered), "")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("want ErrUnsupportedFormat, got %v", err)
	}
	after, _ := os.ReadFile(livePath)
	if !bytes.Equal(originalBytes, after) {
		t.Fatal("live DB modified despite version mismatch")
	}
}

func bumpManifest(t *testing.T, plainTarGz []byte, newVersion string) []byte {
	t.Helper()
	gz, _ := gzip.NewReader(bytes.NewReader(plainTarGz))
	tr := tar.NewReader(gz)

	var out bytes.Buffer
	gw := gzip.NewWriter(&out)
	tw := tar.NewWriter(gw)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		buf, _ := io.ReadAll(tr)
		if hdr.Name == tarManifestName {
			buf = []byte(`{"version":"` + newVersion + `","cairn_version":"x","created_at":"2026-01-01T00:00:00Z","mode":"bundle_plain","has_key":true,"schema_version":7}`)
			hdr.Size = int64(len(buf))
		}
		tw.WriteHeader(hdr)
		tw.Write(buf)
	}
	tw.Close()
	gw.Close()
	return out.Bytes()
}

func TestPBKDF2Roundtrip(t *testing.T) {
	pt := []byte("hello world payload payload payload")
	var enc bytes.Buffer
	if err := encryptAndWrite(&enc, pt, "passphrase-strong-12chars"); err != nil {
		t.Fatalf("enc: %v", err)
	}
	got, err := decryptBundle(enc.Bytes(), "passphrase-strong-12chars")
	if err != nil {
		t.Fatalf("dec: %v", err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("roundtrip mismatch")
	}
	if _, err := decryptBundle(enc.Bytes(), "wrong"); !errors.Is(err, ErrBadPassphrase) {
		t.Fatalf("want bad passphrase, got %v", err)
	}
}

func TestDetectKind(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want Kind
	}{
		{"sqlite", []byte("SQLite format 3\x00extra"), KindDBOnly},
		{"gzip", []byte{0x1f, 0x8b, 0, 0}, KindBundlePlain},
		{"encrypted", []byte("CRBPxxxxxxxxxxxxxxxx"), KindBundleEncrypted},
		{"junk", []byte("hello"), KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectKind(c.head); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
