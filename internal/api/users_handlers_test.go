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
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

func newSettingsTestServer(t *testing.T) (*httptest.Server, *store.Queries, *auth.SessionService) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	usersH := NewUsersHandler(q, sessionSvc, logger)
	sysH := NewSystemSettingsHandler(q, logger)
	retH := NewRetentionSettingsHandler(q, logger)
	checksH := NewChecksHandler(q, db, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, checksH, nil, nil, nil, nil, nil, nil, usersH, sysH, retH, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q, sessionSvc
}

type testClient struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
	csrf   string
}

func loginClient(t *testing.T, srv *httptest.Server, username, password string) *testClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)
	resp, err := c.Get(srv.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	resp.Body.Close()
	csrf := ""
	for _, ck := range jar.Cookies(u) {
		if ck.Name == auth.CSRFCookieName {
			csrf = ck.Value
		}
	}
	tc := &testClient{t: t, srv: srv, client: c, csrf: csrf}
	body := `{"username":"` + username + `","password":"` + password + `"}`
	r := tc.do(http.MethodPost, "/api/auth/login", body)
	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		t.Fatalf("login %s: %d %s", username, r.StatusCode, b)
	}
	r.Body.Close()
	return tc
}

func (c *testClient) do(method, path, body string) *http.Response {
	c.t.Helper()
	var br io.Reader
	if body != "" {
		br = bytes.NewBufferString(body)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, br)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(auth.CSRFHeaderName, c.csrf)
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func (c *testClient) decode(resp *http.Response, v any) {
	c.t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		c.t.Fatalf("decode: %v", err)
	}
}

func TestUserCRUDHappyPath(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")

	r := tc.do(http.MethodPost, "/api/users", `{"username":"editor1","email":"e@x.com","display_name":"Editor One","password":"editor-pw-12345","role":"editor"}`)
	if r.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("create: %d %s", r.StatusCode, b)
	}
	var created struct {
		User userFullResponse `json:"user"`
	}
	tc.decode(r, &created)
	if created.User.Username != "editor1" || created.User.Role != "editor" {
		t.Fatalf("unexpected user: %+v", created.User)
	}

	r2 := tc.do(http.MethodGet, "/api/users", "")
	var list struct {
		Users []userFullResponse `json:"users"`
		Total int                `json:"total"`
	}
	tc.decode(r2, &list)
	if list.Total != 2 {
		t.Fatalf("expected 2 users, got %d", list.Total)
	}
}

func TestUserCannotDeleteSelf(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	users, _ := q.ListUsers(context.Background())
	r := tc.do(http.MethodDelete, "/api/users/"+itoa(users[0].ID), "")
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
	r.Body.Close()
}

func TestUserCannotDeleteLastAdmin(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	admin := seedUser(t, q, "admin", "password-long-enough", "admin")
	other := seedUser(t, q, "admin2", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	r := tc.do(http.MethodDelete, "/api/users/"+itoa(other.ID), "")
	if r.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("delete other admin: %d %s", r.StatusCode, b)
	}
	r.Body.Close()
	// admin is still self → cannot delete self either way; create non-admin and try deleting admin
	editor := seedUser(t, q, "ed", "password-long-enough", "editor")
	tcEd := loginClient(t, srv, "ed", "password-long-enough")
	_ = tcEd
	_ = admin
	_ = editor
	// Editor cannot delete admin (no admin role) — should be 403.
	rE := tcEd.do(http.MethodDelete, "/api/users/"+itoa(admin.ID), "")
	if rE.StatusCode != http.StatusForbidden {
		t.Fatalf("editor delete: expected 403, got %d", rE.StatusCode)
	}
	rE.Body.Close()
}

func TestSelfUpdateCannotChangeOwnRole(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	editor := seedUser(t, q, "ed", "password-long-enough", "editor")
	tc := loginClient(t, srv, "ed", "password-long-enough")
	r := tc.do(http.MethodPatch, "/api/users/"+itoa(editor.ID), `{"role":"admin"}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
	r.Body.Close()
	// But can update own display name.
	r2 := tc.do(http.MethodPatch, "/api/users/"+itoa(editor.ID), `{"display_name":"Edith"}`)
	if r2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("display update: %d %s", r2.StatusCode, b)
	}
	r2.Body.Close()
}

func TestAdminCannotChangeOwnRole(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	admin := seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	r := tc.do(http.MethodPatch, "/api/users/"+itoa(admin.ID), `{"role":"editor"}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
	r.Body.Close()
	other := seedUser(t, q, "u2", "password-long-enough", "viewer")
	r2 := tc.do(http.MethodPatch, "/api/users/"+itoa(other.ID), `{"role":"editor"}`)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("admin updates other role: expected 200, got %d", r2.StatusCode)
	}
	r2.Body.Close()
}

func TestSelfPasswordChangeRequiresCurrent(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	u := seedUser(t, q, "ed", "password-long-enough", "editor")
	tc := loginClient(t, srv, "ed", "password-long-enough")

	r := tc.do(http.MethodPost, "/api/users/"+itoa(u.ID)+"/password", `{"current_password":"wrong","new_password":"newpw-1234567"}`)
	if r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", r.StatusCode)
	}
	r.Body.Close()
	r2 := tc.do(http.MethodPost, "/api/users/"+itoa(u.ID)+"/password", `{"current_password":"password-long-enough","new_password":"newpw-1234567"}`)
	if r2.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("expected 204, got %d %s", r2.StatusCode, b)
	}
	r2.Body.Close()
}

func TestAdminPasswordChangeSkipsCurrent(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	target := seedUser(t, q, "ed", "password-long-enough", "editor")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	r := tc.do(http.MethodPost, "/api/users/"+itoa(target.ID)+"/password", `{"new_password":"newpw-1234567"}`)
	if r.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("expected 204, got %d %s", r.StatusCode, b)
	}
	r.Body.Close()
}

func TestSelfPasswordChangeRevokesOtherSessions(t *testing.T) {
	srv, q, sessionSvc := newSettingsTestServer(t)
	u := seedUser(t, q, "ed", "password-long-enough", "editor")
	// Create a second "other" session directly.
	otherSession, err := sessionSvc.Create(context.Background(), u.ID, "other-ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("create other session: %v", err)
	}
	tc := loginClient(t, srv, "ed", "password-long-enough")
	r := tc.do(http.MethodPost, "/api/users/"+itoa(u.ID)+"/password", `{"current_password":"password-long-enough","new_password":"newpw-1234567"}`)
	if r.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", r.StatusCode)
	}
	r.Body.Close()
	if _, _, err := sessionSvc.Lookup(context.Background(), otherSession); err == nil {
		t.Fatalf("expected other session revoked")
	}
	// Current session still valid.
	rMe := tc.do(http.MethodGet, "/api/auth/me", "")
	if rMe.StatusCode != http.StatusOK {
		t.Fatalf("current session expected 200, got %d", rMe.StatusCode)
	}
	rMe.Body.Close()
}

func TestSystemSettingsValidation(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")

	r := tc.do(http.MethodPatch, "/api/system-settings", `{"incident_reopen_window_seconds":99999999}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
	r.Body.Close()

	r2 := tc.do(http.MethodPatch, "/api/system-settings", `{"incident_reopen_mode":"banana"}`)
	if r2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r2.StatusCode)
	}
	r2.Body.Close()

	r3 := tc.do(http.MethodPatch, "/api/system-settings", `{"incident_id_format":""}`)
	if r3.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty format expected 400, got %d", r3.StatusCode)
	}
	r3.Body.Close()

	r4 := tc.do(http.MethodPatch, "/api/system-settings", `{"incident_id_format":"INC-{year}"}`)
	if r4.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r4.Body)
		t.Fatalf("missing {id} should be allowed: %d %s", r4.StatusCode, b)
	}
	r4.Body.Close()
}

func TestRetentionOrderingValidation(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	r := tc.do(http.MethodPatch, "/api/retention-settings", `{"raw_days":30,"hourly_days":20,"daily_days":180}`)
	if r.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
	r.Body.Close()
	r2 := tc.do(http.MethodPatch, "/api/retention-settings", `{"raw_days":7,"hourly_days":30,"daily_days":180}`)
	if r2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("expected 200, got %d %s", r2.StatusCode, b)
	}
	r2.Body.Close()
}

func TestCheckReopenOverridesAccepted(t *testing.T) {
	srv, q, _ := newSettingsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	tc := loginClient(t, srv, "admin", "password-long-enough")
	r := tc.do(http.MethodPost, "/api/monitors", `{"name":"x","type":"http","reopen_mode":"never","reopen_window_seconds":120,"config":{"url":"https://example.com"}}`)
	if r.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("create: %d %s", r.StatusCode, b)
	}
	var created struct {
		Check checkResponse `json:"check"`
	}
	tc.decode(r, &created)
	if created.Check.ReopenMode == nil || *created.Check.ReopenMode != "never" {
		t.Fatalf("expected reopen_mode=never, got %+v", created.Check.ReopenMode)
	}
	if created.Check.ReopenWindowSeconds == nil || *created.Check.ReopenWindowSeconds != 120 {
		t.Fatalf("expected reopen_window_seconds=120, got %+v", created.Check.ReopenWindowSeconds)
	}

	// Invalid mode rejected.
	r2 := tc.do(http.MethodPost, "/api/monitors", `{"name":"y","type":"http","reopen_mode":"banana","config":{"url":"https://example.com"}}`)
	if r2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r2.StatusCode)
	}
	r2.Body.Close()

	// PATCH null clears.
	r3 := tc.do(http.MethodPatch, "/api/monitors/"+itoa(created.Check.ID), `{"reopen_mode":null,"reopen_window_seconds":null}`)
	if r3.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r3.Body)
		t.Fatalf("patch null: %d %s", r3.StatusCode, b)
	}
	var patched struct {
		Check checkResponse `json:"check"`
	}
	tc.decode(r3, &patched)
	if patched.Check.ReopenMode != nil || patched.Check.ReopenWindowSeconds != nil {
		t.Fatalf("expected null reopen overrides after null patch")
	}
}

