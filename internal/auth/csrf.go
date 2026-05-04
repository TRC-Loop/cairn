// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

const (
	CSRFCookieName   = "cairn_csrf"
	CSRFHeaderName   = "X-CSRF-Token"
	csrfCookieMaxAge = 7 * 24 * 60 * 60
)

// csrfExemptPrefixes are paths that skip CSRF checks.
// /push/* heartbeats come from external systems without browser context.
// /p/{slug}/unlock has its own HMAC cookie guard.
var csrfExemptPrefixes = []string{"/healthz", "/readyz", "/push/", "/api/setup/"}

var csrfExemptExact = []string{"/healthz", "/readyz", "/api/auth/login"}

func isCSRFExempt(path string) bool {
	for _, p := range csrfExemptExact {
		if path == p {
			return true
		}
	}
	for _, p := range csrfExemptPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	if strings.HasPrefix(path, "/p/") && strings.HasSuffix(path, "/unlock") {
		return true
	}
	return false
}

func CSRFMiddleware(logger *slog.Logger, behindTLS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isCSRFExempt(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				if _, err := r.Cookie(CSRFCookieName); err != nil {
					tok, terr := GenerateSessionID()
					if terr != nil {
						logger.Error("csrf token generate failed", "err", terr)
						http.Error(w, "internal error", http.StatusInternalServerError)
						return
					}
					http.SetCookie(w, &http.Cookie{
						Name:     CSRFCookieName,
						Value:    tok,
						Path:     "/",
						MaxAge:   csrfCookieMaxAge,
						HttpOnly: false,
						Secure:   behindTLS,
						SameSite: http.SameSiteLaxMode,
					})
				}
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(CSRFCookieName)
			if err != nil || cookie.Value == "" {
				logger.Warn("csrf cookie missing", "path", r.URL.Path, "method", r.Method)
				writeCSRFError(w)
				return
			}
			header := r.Header.Get(CSRFHeaderName)
			if header == "" {
				logger.Warn("csrf header missing", "path", r.URL.Path, "method", r.Method)
				writeCSRFError(w)
				return
			}
			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				logger.Warn("csrf mismatch", "path", r.URL.Path, "method", r.Method)
				writeCSRFError(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"code":"forbidden","description":"csrf token missing or invalid"}`))
}
