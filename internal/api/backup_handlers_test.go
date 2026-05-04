// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/backup"
)

func TestBackupDownloadAdminVsRoles(t *testing.T) {
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	dbPath := filepath.Join(t.TempDir(), "live.db")
	svc := backup.NewService(db, dbPath, "test-master-key-32bytes-padding!!", "test", logger)
	bh := NewBackupHandler(svc, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, bh, nil, false))
	defer srv.Close()

	seedUser(t, q, "admin", "password-long-enough", "admin")
	seedUser(t, q, "editor", "password-long-enough", "editor")
	seedUser(t, q, "viewer", "password-long-enough", "viewer")

	adminClient := loginAs(t, srv, "admin", "password-long-enough")
	editorClient := loginAs(t, srv, "editor", "password-long-enough")
	viewerClient := loginAs(t, srv, "viewer", "password-long-enough")

	// admin db_only succeeds, returns SQLite file
	resp := doJSON(t, adminClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "db_only"})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("admin db_only: status %d body=%s", resp.StatusCode, body)
	}
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if backup.DetectKind(got) != backup.KindDBOnly {
		t.Fatalf("expected SQLite payload, kind=%v len=%d", backup.DetectKind(got), len(got))
	}

	// editor → 403
	resp = doJSON(t, editorClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "db_only"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("editor: expected 403, got %d", resp.StatusCode)
	}

	// viewer → 403
	resp = doJSON(t, viewerClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "db_only"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("viewer: expected 403, got %d", resp.StatusCode)
	}

	// encrypted no passphrase → 400
	resp = doJSON(t, adminClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "bundle_encrypted"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("encrypted no pass: expected 400, got %d", resp.StatusCode)
	}

	// encrypted valid → 200, magic header
	resp = doJSON(t, adminClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "bundle_encrypted", "passphrase": "passphrase-12-chars-min"})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("encrypted valid: %d body=%s", resp.StatusCode, body)
	}
	got, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.HasPrefix(got, []byte(backup.BundleMagic)) {
		t.Fatalf("expected magic header, got %q", got[:8])
	}

	// plain → 200, gzip header
	resp = doJSON(t, adminClient, srv, http.MethodPost, "/api/backup/download", map[string]string{"mode": "bundle_plain"})
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("plain: %d body=%s", resp.StatusCode, body)
	}
	got, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if backup.DetectKind(got) != backup.KindBundlePlain {
		t.Fatalf("expected gzip payload, kind=%v", backup.DetectKind(got))
	}
}
