// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func csrfTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCSRFGetSetsCookie(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == CSRFCookieName && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("CSRF cookie not set on GET")
	}
}

func TestCSRFPostMissingHeader(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/anything", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "abc"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCSRFPostMismatch(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/anything", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "abc"})
	req.Header.Set(CSRFHeaderName, "xyz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestCSRFPostMatch(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/anything", strings.NewReader(""))
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "abc"})
	req.Header.Set(CSRFHeaderName, "abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCSRFExemptPush(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/push/sometoken", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exempt path, got %d", rec.Code)
	}
}

func TestCSRFExemptLogin(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(""))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exempt login, got %d", rec.Code)
	}
}

func TestCSRFExemptSetup(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/setup/complete", strings.NewReader(""))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exempt setup, got %d", rec.Code)
	}
}

func TestCSRFExemptStatusPageUnlock(t *testing.T) {
	h := CSRFMiddleware(testLogger(), false)(csrfTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/p/main/unlock", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exempt path, got %d", rec.Code)
	}
}
