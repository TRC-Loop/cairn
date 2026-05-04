// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
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
	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/pquerna/otp/totp"
)

const test2FAEncKey = "test-encryption-key-32-bytes-long!"

func newTwoFATestServer(t *testing.T) (*httptest.Server, *store.Queries, *crypto.SecretBox, *http.Client, store.User) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	box, err := crypto.NewSecretBox(test2FAEncKey)
	if err != nil {
		t.Fatalf("box: %v", err)
	}
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	usersH := NewUsersHandler(q, sessionSvc, logger)
	twoH := NewTwoFAHandler(q, sessionSvc, box, logger, false)
	authH.SetTwoFA(twoH)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, nil, nil, nil, nil, usersH, nil, nil, nil, twoH, false, "dev", "unknown"))
	t.Cleanup(srv.Close)

	user := seedUser(t, q, "alice", "password-long-enough", "admin")
	client := loginAs(t, srv, "alice", "password-long-enough")
	return srv, q, box, client, user
}

func TestTwoFASetupReturnsSecretAndQR(t *testing.T) {
	srv, q, _, client, user := newTwoFATestServer(t)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/setup", nil)
	defer r.Body.Close()
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status %d: %s", r.StatusCode, b)
	}
	var resp setupResponse
	_ = json.NewDecoder(r.Body).Decode(&resp)
	if resp.Secret == "" || !strings.HasPrefix(resp.OtpauthURL, "otpauth://") || !strings.HasPrefix(resp.QRCodeDataURL, "data:image/png;base64,") {
		t.Fatalf("bad response: %+v", resp)
	}
	got, err := q.GetUserByID(context.Background(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.TotpSecretEnc.Valid || got.TotpEnabled {
		t.Fatalf("user state wrong: %+v", got)
	}
}

func TestTwoFASetupRejectedWhenAlreadyEnabled(t *testing.T) {
	srv, q, box, client, user := newTwoFATestServer(t)
	enableTOTP(t, q, box, user.ID)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/setup", nil)
	r.Body.Close()
	if r.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", r.StatusCode)
	}
}

func TestTwoFAConfirmEnablesAndReturnsCodes(t *testing.T) {
	srv, q, _, client, user := newTwoFATestServer(t)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/setup", nil)
	var setup setupResponse
	_ = json.NewDecoder(r.Body).Decode(&setup)
	r.Body.Close()
	code, _ := totp.GenerateCode(setup.Secret, time.Now().UTC())
	r2 := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/confirm", map[string]string{"code": code})
	defer r2.Body.Close()
	if r2.StatusCode != 200 {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("confirm %d: %s", r2.StatusCode, b)
	}
	var rr recoveryCodesResponse
	_ = json.NewDecoder(r2.Body).Decode(&rr)
	if len(rr.RecoveryCodes) != 10 {
		t.Fatalf("expected 10 codes, got %d", len(rr.RecoveryCodes))
	}
	got, _ := q.GetUserByID(context.Background(), user.ID)
	if !got.TotpEnabled || !got.TotpEnrolledAt.Valid {
		t.Fatalf("user not enabled: %+v", got)
	}
}

func TestTwoFAConfirmInvalidCode(t *testing.T) {
	srv, q, _, client, user := newTwoFATestServer(t)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/setup", nil)
	r.Body.Close()
	r2 := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/confirm", map[string]string{"code": "000000"})
	r2.Body.Close()
	if r2.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", r2.StatusCode)
	}
	got, _ := q.GetUserByID(context.Background(), user.ID)
	if got.TotpEnabled {
		t.Fatal("user should not be enabled after bad confirm")
	}
}

func TestTwoFADisableRequiresPasswordAndCode(t *testing.T) {
	srv, q, box, client, user := newTwoFATestServer(t)
	secret := enableTOTP(t, q, box, user.ID)

	bad := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/disable",
		map[string]string{"current_password": "wrong", "code": totpNow(t, secret)})
	bad.Body.Close()
	if bad.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", bad.StatusCode)
	}
	ok := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/disable",
		map[string]string{"current_password": "password-long-enough", "code": totpNow(t, secret)})
	ok.Body.Close()
	if ok.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", ok.StatusCode)
	}
	got, _ := q.GetUserByID(context.Background(), user.ID)
	if got.TotpEnabled || got.TotpSecretEnc.Valid {
		t.Fatalf("user state wrong: %+v", got)
	}
	codes, _ := q.ListUnusedRecoveryCodesForUser(context.Background(), user.ID)
	if len(codes) != 0 {
		t.Fatalf("recovery codes not deleted: %d", len(codes))
	}
}

func TestTwoFARegenerateRecoveryCodes(t *testing.T) {
	srv, q, box, client, user := newTwoFATestServer(t)
	secret := enableTOTP(t, q, box, user.ID)
	old, _ := q.ListUnusedRecoveryCodesForUser(context.Background(), user.ID)
	if len(old) == 0 {
		t.Fatal("no initial codes")
	}
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/2fa/regenerate-recovery-codes",
		map[string]string{"current_password": "password-long-enough", "code": totpNow(t, secret)})
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	var rr recoveryCodesResponse
	_ = json.NewDecoder(r.Body).Decode(&rr)
	if len(rr.RecoveryCodes) != 10 {
		t.Fatalf("expected 10, got %d", len(rr.RecoveryCodes))
	}
	now, _ := q.ListUnusedRecoveryCodesForUser(context.Background(), user.ID)
	if len(now) != 10 {
		t.Fatalf("expected 10 in db, got %d", len(now))
	}
	for _, o := range old {
		for _, n := range now {
			if o.ID == n.ID {
				t.Fatalf("old recovery code id %d still present", o.ID)
			}
		}
	}
}

func TestLoginIssuesChallengeWhen2FAEnabled(t *testing.T) {
	srv, q, box, _, user := newTwoFATestServer(t)
	enableTOTP(t, q, box, user.ID)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)
	prime, _ := client.Get(srv.URL + "/api/auth/me")
	prime.Body.Close()
	csrf := ""
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName {
			csrf = c.Value
		}
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login", strings.NewReader(`{"username":"alice","password":"password-long-enough"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeaderName, csrf)
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body["requires_2fa"] != true {
		t.Fatalf("missing requires_2fa: %v", body)
	}
	if tok, _ := body["challenge_token"].(string); tok == "" {
		t.Fatal("missing challenge_token")
	}
	for _, c := range r.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			t.Fatal("session cookie should not be set yet")
		}
	}
}

func TestLoginVerifyTOTPCode(t *testing.T) {
	srv, q, box, _, user := newTwoFATestServer(t)
	secret := enableTOTP(t, q, box, user.ID)
	tok, client := loginAndChallenge(t, srv)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/login/2fa",
		map[string]string{"challenge_token": tok, "code": totpNow(t, secret)})
	defer r.Body.Close()
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		t.Fatalf("status %d: %s", r.StatusCode, b)
	}
	hasSession := false
	for _, c := range r.Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			hasSession = true
		}
	}
	if !hasSession {
		t.Fatal("session cookie not set")
	}
	_ = user
}

func TestLoginVerifyRecoveryCodeMarksUsed(t *testing.T) {
	srv, q, box, _, user := newTwoFATestServer(t)
	enableTOTP(t, q, box, user.ID)
	plain := "AAA-BBB-CCC-DDD"
	hash, _ := auth.Hash(plain)
	rc, err := q.CreateRecoveryCode(context.Background(), store.CreateRecoveryCodeParams{UserID: user.ID, CodeHash: hash})
	if err != nil {
		t.Fatal(err)
	}

	tok, client := loginAndChallenge(t, srv)
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/login/2fa",
		map[string]string{"challenge_token": tok, "code": plain})
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("status %d", r.StatusCode)
	}
	got, _ := q.ListUnusedRecoveryCodesForUser(context.Background(), user.ID)
	for _, c := range got {
		if c.ID == rc.ID {
			t.Fatal("recovery code still unused")
		}
	}

	tok2, client2 := loginAndChallenge(t, srv)
	r2 := doJSON(t, client2, srv, http.MethodPost, "/api/auth/login/2fa",
		map[string]string{"challenge_token": tok2, "code": plain})
	r2.Body.Close()
	if r2.StatusCode != 401 {
		t.Fatalf("expected 401 on reuse, got %d", r2.StatusCode)
	}
}

func TestLoginVerify3StrikesInvalidatesToken(t *testing.T) {
	srv, q, box, _, user := newTwoFATestServer(t)
	enableTOTP(t, q, box, user.ID)
	tok, client := loginAndChallenge(t, srv)
	for i := 0; i < 2; i++ {
		r := doJSON(t, client, srv, http.MethodPost, "/api/auth/login/2fa",
			map[string]string{"challenge_token": tok, "code": "000000"})
		r.Body.Close()
		if r.StatusCode != 401 {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, r.StatusCode)
		}
	}
	r := doJSON(t, client, srv, http.MethodPost, "/api/auth/login/2fa",
		map[string]string{"challenge_token": tok, "code": "000000"})
	defer r.Body.Close()
	if r.StatusCode != 401 {
		t.Fatalf("3rd: expected 401, got %d", r.StatusCode)
	}
	body, _ := io.ReadAll(r.Body)
	if !strings.Contains(string(body), "too many attempts") {
		t.Fatalf("expected token invalidation message, got %s", body)
	}
}

func TestAdminResetTwoFA(t *testing.T) {
	srv, q, box, client, _ := newTwoFATestServer(t)
	target := seedUser(t, q, "bob", "password-long-enough", "viewer")
	enableTOTP(t, q, box, target.ID)

	r := doJSON(t, client, srv, http.MethodPost,
		"/api/users/"+itoa(target.ID)+"/reset-2fa", nil)
	r.Body.Close()
	if r.StatusCode != 204 {
		t.Fatalf("status %d", r.StatusCode)
	}
	got, _ := q.GetUserByID(context.Background(), target.ID)
	if got.TotpEnabled || got.TotpSecretEnc.Valid {
		t.Fatalf("not reset: %+v", got)
	}
	codes, _ := q.ListUnusedRecoveryCodesForUser(context.Background(), target.ID)
	if len(codes) != 0 {
		t.Fatalf("codes remaining: %d", len(codes))
	}
}

func TestAdminResetForbiddenForViewer(t *testing.T) {
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	box, _ := crypto.NewSecretBox(test2FAEncKey)
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	usersH := NewUsersHandler(q, sessionSvc, logger)
	twoH := NewTwoFAHandler(q, sessionSvc, box, logger, false)
	authH.SetTwoFA(twoH)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, nil, nil, nil, nil, usersH, nil, nil, nil, twoH, false, "dev", "unknown"))
	defer srv.Close()

	seedUser(t, q, "vw", "password-long-enough", "viewer")
	target := seedUser(t, q, "bob", "password-long-enough", "viewer")
	client := loginAs(t, srv, "vw", "password-long-enough")
	r := doJSON(t, client, srv, http.MethodPost,
		"/api/users/"+itoa(target.ID)+"/reset-2fa", nil)
	r.Body.Close()
	if r.StatusCode != 403 {
		t.Fatalf("expected 403, got %d", r.StatusCode)
	}
}

// ---- helpers ----

func enableTOTP(t *testing.T, q *store.Queries, box *crypto.SecretBox, userID int64) string {
	t.Helper()
	secret, _, err := auth.GenerateTOTPSecret("test")
	if err != nil {
		t.Fatal(err)
	}
	enc, err := box.EncryptString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.SetUserTOTPSecret(context.Background(), store.SetUserTOTPSecretParams{
		TotpSecretEnc: sql.NullString{String: enc, Valid: true}, ID: userID,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.EnableUserTOTP(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	_, hashes, err := auth.GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hashes {
		if _, err := q.CreateRecoveryCode(context.Background(), store.CreateRecoveryCodeParams{UserID: userID, CodeHash: h}); err != nil {
			t.Fatal(err)
		}
	}
	return secret
}

func totpNow(t *testing.T, secret string) string {
	t.Helper()
	c, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// loginAndChallenge logs in a 2FA-enabled user (alice) and returns the challenge
// token plus a fresh client whose cookie jar already has the CSRF cookie.
func loginAndChallenge(t *testing.T, srv *httptest.Server) (string, *http.Client) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	u, _ := url.Parse(srv.URL)
	prime, _ := client.Get(srv.URL + "/api/auth/me")
	prime.Body.Close()
	csrf := ""
	for _, c := range jar.Cookies(u) {
		if c.Name == auth.CSRFCookieName {
			csrf = c.Value
		}
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/login",
		strings.NewReader(`{"username":"alice","password":"password-long-enough"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeaderName, csrf)
	r, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	tok, _ := body["challenge_token"].(string)
	if tok == "" {
		t.Fatalf("no challenge_token in %v", body)
	}
	return tok, client
}
