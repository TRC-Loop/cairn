// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

func newSetupTestServer(t *testing.T) (*httptest.Server, *store.Queries) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	setupH := NewSetupHandler(q, db, sessionSvc, logger, false)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, setupH, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q
}

func TestSetupStatusFreshDB(t *testing.T) {
	srv, _ := newSetupTestServer(t)
	resp, err := http.Get(srv.URL + "/api/setup/status")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]bool
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["setup_complete"] {
		t.Fatalf("expected setup_complete=false on fresh db")
	}
}

func TestSetupStatusWithUser(t *testing.T) {
	srv, q := newSetupTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	resp, err := http.Get(srv.URL + "/api/setup/status")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]bool
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !body["setup_complete"] {
		t.Fatal("expected setup_complete=true after user exists")
	}
}

func TestSetupCompleteSuccess(t *testing.T) {
	srv, q := newSetupTestServer(t)
	payload := `{
		"username":"admin",
		"email":"admin@example.com",
		"display_name":"Admin",
		"password":"super-secret-password"
	}`
	resp, err := http.Post(srv.URL+"/api/setup/complete", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if _, ok := body["user"]; !ok {
		t.Fatal("missing user in response")
	}
	if _, ok := body["status_page"]; ok {
		t.Fatal("status_page should not be in response")
	}
	hasSession := false
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatal("session cookie not set after setup")
	}
	users, _ := q.ListUsers(context.Background())
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	pages, _ := q.ListStatusPages(context.Background())
	if len(pages) != 0 {
		t.Fatalf("expected 0 status pages, got %d", len(pages))
	}
}

func TestSetupCompleteConflict(t *testing.T) {
	srv, q := newSetupTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	payload := `{"username":"other","email":"o@example.com","display_name":"O","password":"super-secret-password"}`
	resp, err := http.Post(srv.URL+"/api/setup/complete", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestSetupCompleteValidation(t *testing.T) {
	srv, _ := newSetupTestServer(t)
	cases := []struct {
		name  string
		body  string
		field string
	}{
		{"short password", `{"username":"admin","email":"a@b.co","display_name":"A","password":"short"}`, "password"},
		{"bad email", `{"username":"admin","email":"not-an-email","display_name":"A","password":"super-secret-password"}`, "email"},
		{"bad username", `{"username":"a b","email":"a@b.co","display_name":"A","password":"super-secret-password"}`, "username"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(srv.URL+"/api/setup/complete", "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
			var body map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&body)
			fields, _ := body["fields"].(map[string]any)
			if _, ok := fields[tc.field]; !ok {
				t.Fatalf("expected field error for %q, got %v", tc.field, fields)
			}
		})
	}
}

func TestLoginNoCSRFSucceeds(t *testing.T) {
	srv, q := newSetupTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", strings.NewReader(`{"username":"admin","password":"password-long-enough"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 without csrf, got %d: %s", resp.StatusCode, b)
	}
}
