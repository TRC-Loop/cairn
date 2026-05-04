// SPDX-License-Identifier: AGPL-3.0-or-later
package auth

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/TRC-Loop/cairn/internal/store"
)

const SessionCookieName = "cairn_session"

type contextKey int

const (
	ctxKeyUser contextKey = iota
	ctxKeySession
)

func RequireAuth(svc *SessionService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				writeAuthError(w)
				return
			}
			sess, user, err := svc.Lookup(r.Context(), cookie.Value)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, ErrSessionExpired) {
					logger.Warn("session lookup failed", "err", err)
				}
				clearSessionCookie(w)
				writeAuthError(w)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUser, user)
			ctx = context.WithValue(ctx, ctxKeySession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuth(svc *SessionService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			sess, user, err := svc.Lookup(r.Context(), cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUser, user)
			ctx = context.WithValue(ctx, ctxKeySession, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(ctxKeyUser).(store.User)
	return u, ok
}

func SessionFromContext(ctx context.Context) (store.Session, bool) {
	s, ok := ctx.Value(ctxKeySession).(store.Session)
	return s, ok
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeAuthError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"code":"unauthorized","description":"not authenticated"}`))
}
