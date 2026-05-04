// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, ok := UserFromContext(r.Context()); ok {
			w.Header().Set("X-User", u.Username)
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireAuthNoCookie(t *testing.T) {
	_, q := openTestDB(t)
	svc := NewSessionService(q, testLogger())
	h := RequireAuth(svc, testLogger())(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAuthValidCookie(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "hannah", "password-long-enough", "admin")
	svc := NewSessionService(q, testLogger())
	id, err := svc.Create(context.Background(), u.ID, "", "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	h := RequireAuth(svc, testLogger())(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: id})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-User") != "hannah" {
		t.Fatalf("user not in ctx")
	}
}

func TestRequireAuthExpiredClearsCookie(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "ivy", "password-long-enough", "admin")
	svc := NewSessionService(q, testLogger())
	t0 := time.Now().UTC()
	svc.now = func() time.Time { return t0 }
	id, _ := svc.Create(context.Background(), u.ID, "", "")
	svc.now = func() time.Time { return t0.Add(DefaultSessionTTL + time.Hour) }

	h := RequireAuth(svc, testLogger())(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: id})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	setCookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, SessionCookieName+"=;") && !strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("expected cleared session cookie, got %q", setCookie)
	}
}

func TestRequireRoleRejectsLower(t *testing.T) {
	_, q := openTestDB(t)
	u := createTestUser(t, q, "joe", "password-long-enough", "viewer")
	svc := NewSessionService(q, testLogger())
	id, _ := svc.Create(context.Background(), u.ID, "", "")

	chain := RequireAuth(svc, testLogger())(
		RequireRole(RoleEditor, testLogger())(okHandler()),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: id})
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}
