// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

func TestPushEndpointSuccess(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()
	token := "test-token-abc"

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "hb",
		Type:              "push",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  1,
		RecoveryThreshold: 1,
		ConfigJson:        `{"grace_period_seconds":60}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := q.SetCheckPushToken(ctx, store.SetCheckPushTokenParams{
		PushToken: sql.NullString{String: token, Valid: true},
		ID:        c.ID,
	}); err != nil {
		t.Fatalf("set token: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/push/"+token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body)
	}

	rows, err := q.GetRecentResults(ctx, store.GetRecentResultsParams{CheckID: c.ID, Limit: 10})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Status != "up" {
		t.Fatalf("expected up, got %s", rows[0].Status)
	}

	updated, err := q.GetCheck(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if updated.LastStatus != "up" {
		t.Fatalf("expected last_status=up, got %s", updated.LastStatus)
	}
}

func TestPushEndpointUnknownToken(t *testing.T) {
	db, q := openTestDB(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/push/does-not-exist", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
