// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

func seedUser(t *testing.T, q *store.Queries, username, password, role string) store.User {
	t.Helper()
	hash, err := auth.Hash(password)
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

func newAuthTestServer(t *testing.T) (*httptest.Server, *store.Queries) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	h := NewAuthHandler(q, sessionSvc, logger, false)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, h, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false))
	t.Cleanup(srv.Close)
	return srv, q
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "t"})
	req.Header.Set(auth.CSRFHeaderName, "t")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return resp
}

func TestLoginSuccess(t *testing.T) {
	srv, q := newAuthTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")

	resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"admin","password":"password-long-enough"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	user, _ := body["user"].(map[string]any)
	if user["username"] != "admin" {
		t.Fatalf("unexpected user: %v", body)
	}
	hasSession := false
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatal("session cookie not set")
	}
	sessions, err := q.ListSessionsForUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
}

func TestLoginWrongPassword(t *testing.T) {
	srv, q := newAuthTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"admin","password":"wrong-password-here"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "invalid credentials") {
		t.Fatalf("expected generic error, got %s", b)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	srv, _ := newAuthTestServer(t)
	resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"ghost","password":"password-long-enough"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "invalid credentials") {
		t.Fatalf("expected same generic error, got %s", b)
	}
}

func TestLoginRateLimit(t *testing.T) {
	srv, q := newAuthTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")

	for i := 0; i < 5; i++ {
		resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"admin","password":"wrong-password"}`)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d expected 401, got %d", i+1, resp.StatusCode)
		}
	}
	resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"admin","password":"wrong-password"}`)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 6th, got %d", resp.StatusCode)
	}
}

func TestLogoutAndIdempotent(t *testing.T) {
	srv, q := newAuthTestServer(t)
	u := seedUser(t, q, "admin", "password-long-enough", "admin")

	// First, log in to get a session cookie.
	resp := postJSON(t, srv.URL+"/api/auth/login", `{"username":"admin","password":"password-long-enough"}`)
	resp.Body.Close()
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie")
	}

	// Logout with cookie + csrf.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	req.AddCookie(sessionCookie)
	req.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "t"})
	req.Header.Set(auth.CSRFHeaderName, "t")
	logoutResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", logoutResp.StatusCode)
	}
	sessions, _ := q.ListSessionsForUser(context.Background(), u.ID)
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions after logout, got %d", len(sessions))
	}

	// Idempotent: call again without a cookie.
	req2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	req2.AddCookie(&http.Cookie{Name: auth.CSRFCookieName, Value: "t"})
	req2.Header.Set(auth.CSRFHeaderName, "t")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("logout2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 idempotent, got %d", resp2.StatusCode)
	}
}

func TestMeUnauthenticated(t *testing.T) {
	srv, _ := newAuthTestServer(t)
	resp, err := http.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestFullAuthFlow(t *testing.T) {
	srv, q := newAuthTestServer(t)
	_ = seedUser(t, q, "admin", "password-long-enough", "admin")

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)

	// Step 1: prime CSRF cookie via GET.
	primeResp, err := client.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	primeResp.Body.Close()
	csrf := ""
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName {
			csrf = c.Value
		}
	}
	if csrf == "" {
		t.Fatal("no csrf cookie")
	}

	// Step 2: login.
	loginReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login",
		bytes.NewBufferString(`{"username":"admin","password":"password-long-enough"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(auth.CSRFHeaderName, csrf)
	loginResp, err := client.Do(loginReq)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login expected 200, got %d", loginResp.StatusCode)
	}

	// Step 3: /me.
	meResp, err := client.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	body, _ := io.ReadAll(meResp.Body)
	meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("me expected 200, got %d body=%s", meResp.StatusCode, body)
	}

	// Step 4: logout.
	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	logoutReq.Header.Set(auth.CSRFHeaderName, csrf)
	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout expected 204, got %d", logoutResp.StatusCode)
	}

	// Step 5: /me again — should be 401 since session was revoked.
	me2Resp, err := client.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("me2: %v", err)
	}
	me2Resp.Body.Close()
	if me2Resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me2 expected 401, got %d", me2Resp.StatusCode)
	}
}
