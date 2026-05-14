// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

func newChecksTestServer(t *testing.T) (*httptest.Server, *store.Queries, *sql.DB) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	checksH := NewChecksHandler(q, db, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, checksH, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q, db
}

func loginAs(t *testing.T, srv *httptest.Server, username, password string) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)

	// Prime CSRF
	resp, err := client.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	resp.Body.Close()
	csrf := ""
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("no csrf cookie")
	}

	body := `{"username":"` + username + `","password":"` + password + `"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeaderName, csrf)
	loginResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login expected 200, got %d", loginResp.StatusCode)
	}
	return client
}

func doJSON(t *testing.T, client *http.Client, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, srv.URL+path, buf)
	req.Header.Set("Content-Type", "application/json")
	u, _ := url.Parse(srv.URL)
	for _, c := range client.Jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName {
			req.Header.Set(auth.CSRFHeaderName, c.Value)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func TestChecksListUnauthenticated(t *testing.T) {
	srv, _, _ := newChecksTestServer(t)
	resp, err := http.Get(srv.URL + "/api/checks")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestChecksListAuthenticated(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	_, _ = q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "one", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{"url":"https://x"}`,
	})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/checks", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	checks, _ := out["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
}

func TestChecksGetNotFound(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/checks/999", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestChecksCreate(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")

	body := map[string]any{
		"name":   "API",
		"type":   "http",
		"config": map[string]any{"url": "https://example.com"},
	}
	resp := doJSON(t, client, srv, http.MethodPost, "/api/checks", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	ch, _ := out["check"].(map[string]any)
	if ch["type"] != "http" {
		t.Fatalf("unexpected: %v", out)
	}
}

func TestChecksCreateInvalidType(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/checks", map[string]any{
		"name": "X", "type": "bogus",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChecksCreateAsViewerForbidden(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "viewer", "password-long-enough", "viewer")
	client := loginAs(t, srv, "viewer", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/checks", map[string]any{
		"name": "X", "type": "http", "config": map[string]any{"url": "https://x"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestChecksCreatePushGeneratesToken(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/checks", map[string]any{
		"name": "hb", "type": "push",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	ch, _ := out["check"].(map[string]any)
	tok, ok := ch["push_token"].(string)
	if !ok || tok == "" {
		t.Fatalf("expected push_token, got %v", ch)
	}
}

func TestChecksUpdateTypeChangeRejected(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	c, _ := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{}`,
	})
	newType := "tcp"
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/checks/"+itoa(c.ID), map[string]any{
		"type": newType,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChecksUpdatePartial(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	c, _ := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{}`,
	})
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/checks/"+itoa(c.ID), map[string]any{
		"name": "renamed",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	updated, _ := q.GetCheck(context.Background(), c.ID)
	if updated.Name != "renamed" || updated.IntervalSeconds != 60 {
		t.Fatalf("unexpected: %+v", updated)
	}
}

func TestChecksDeleteIdempotent(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	c, _ := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{}`,
	})
	resp := doJSON(t, client, srv, http.MethodDelete, "/api/checks/"+itoa(c.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	resp2 := doJSON(t, client, srv, http.MethodDelete, "/api/checks/999", nil)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 idempotent, got %d", resp2.StatusCode)
	}
}

func TestChecksRecentResults(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	c, _ := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{}`,
	})
	_ = q.InsertCheckResult(context.Background(), store.InsertCheckResultParams{
		CheckID:   c.ID,
		CheckedAt: time.Now().UTC(),
		Status:    "up",
		LatencyMs: sql.NullInt64{Int64: 42, Valid: true},
	})
	resp := doJSON(t, client, srv, http.MethodGet, "/api/checks/"+itoa(c.ID)+"/results?hours=24", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	results, _ := out["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestChecksViewerSeesNoPushToken(t *testing.T) {
	srv, q, _ := newChecksTestServer(t)
	seedUser(t, q, "viewer", "password-long-enough", "viewer")
	c, _ := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "hb", Type: "push", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 1, RecoveryThreshold: 1, ConfigJson: `{}`,
	})
	_ = q.SetCheckPushToken(context.Background(), store.SetCheckPushTokenParams{
		PushToken: sql.NullString{String: "secret-token", Valid: true}, ID: c.ID,
	})
	client := loginAs(t, srv, "viewer", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/checks/"+itoa(c.ID), nil)
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	ch, _ := out["check"].(map[string]any)
	if _, ok := ch["push_token"]; ok {
		t.Fatalf("viewer should not see push_token: %v", ch)
	}
}

func itoa(i int64) string {
	return intToA(i)
}

func intToA(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
