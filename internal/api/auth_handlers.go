// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

// dummyPasswordHash is verified when the user is not found, so the
// request path spends comparable time whether or not the user exists.
var dummyPasswordHash string

func init() {
	h, err := auth.Hash("not-a-real-password-used-for-timing-only")
	if err != nil {
		// Fall back to empty; Verify will just fail fast. Unlikely in practice.
		return
	}
	dummyPasswordHash = h
}

type AuthHandler struct {
	q         *store.Queries
	sessions  *auth.SessionService
	logger    *slog.Logger
	behindTLS bool
	limiter   *loginLimiter
	twofa     *TwoFAHandler
}

func NewAuthHandler(q *store.Queries, sessions *auth.SessionService, logger *slog.Logger, behindTLS bool) *AuthHandler {
	return &AuthHandler{
		q:         q,
		sessions:  sessions,
		logger:    logger,
		behindTLS: behindTLS,
		limiter:   newLoginLimiter(5, 10*time.Minute),
	}
}

// SetTwoFA wires the 2FA challenge issuer. Called from main when both handlers are constructed.
func (h *AuthHandler) SetTwoFA(t *TwoFAHandler) { h.twofa = t }

type loginRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type userResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	TotpEnabled bool   `json:"totp_enabled"`
}

func toUserResponse(u store.User) userResponse {
	return userResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		TotpEnabled: u.TotpEnabled,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if !h.limiter.allow(ip) {
		h.logger.Warn("login rate limit exceeded", "ip", ip)
		writeError(w, http.StatusTooManyRequests, CodeRateLimited, "too many attempts", nil)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	ident := firstNonEmpty(req.Identifier, req.Username, req.Email)
	if ident == "" || req.Password == "" {
		fields := map[string]string{}
		if ident == "" {
			fields["identifier"] = FieldRequired
		}
		if req.Password == "" {
			fields["password"] = FieldRequired
		}
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "missing credentials", fields)
		return
	}

	user, err := h.lookupUser(r, ident)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.logger.Error("user lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		// Run a dummy verify to keep timing similar to the valid-user path.
		if dummyPasswordHash != "" {
			_, _ = auth.Verify(req.Password, dummyPasswordHash)
		}
		h.limiter.fail(ip)
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials", nil)
		return
	}

	ok, err := auth.Verify(req.Password, user.PasswordHash)
	if err != nil || !ok {
		h.limiter.fail(ip)
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials", nil)
		return
	}

	if user.TotpEnabled && h.twofa != nil {
		tok, err := h.twofa.ChallengeIssue(user.ID)
		if err != nil {
			h.logger.Error("issue 2fa challenge failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		h.limiter.reset(ip)
		h.logger.Info("login 2fa challenge issued", "user_id", user.ID, "ip", ip)
		writeJSON(w, http.StatusOK, map[string]any{"requires_2fa": true, "challenge_token": tok})
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), user.ID, r.UserAgent(), clientIP(r))
	if err != nil {
		h.logger.Error("create session failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   h.behindTLS,
		SameSite: http.SameSiteLaxMode,
	})

	h.limiter.reset(ip)
	h.logger.Info("login", "user_id", user.ID, "ip", ip)
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserResponse(user)})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.SessionCookieName)
	if err == nil && cookie.Value != "" {
		if err := h.sessions.Revoke(r.Context(), cookie.Value); err != nil {
			h.logger.Warn("revoke session failed", "err", err)
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.behindTLS,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "not authenticated", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserResponse(user)})
}

func (h *AuthHandler) lookupUser(r *http.Request, ident string) (store.User, error) {
	if strings.Contains(ident, "@") {
		u, err := h.q.GetUserByEmail(r.Context(), ident)
		if err == nil {
			return u, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return store.User{}, err
		}
	}
	return h.q.GetUserByUsername(r.Context(), ident)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		if i := strings.Index(ip, ","); i >= 0 {
			return strings.TrimSpace(ip[:i])
		}
		return strings.TrimSpace(ip)
	}
	return stripPort(r.RemoteAddr)
}

// loginLimiter is an in-memory per-IP failure counter with sliding window.
type loginLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*limiterEntry
}

type limiterEntry struct {
	count    int
	firstAt  time.Time
	blockedTo time.Time
}

func newLoginLimiter(max int, window time.Duration) *loginLimiter {
	return &loginLimiter{max: max, window: window, entries: map[string]*limiterEntry{}}
}

func (l *loginLimiter) allow(ip string) bool {
	if ip == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.gc()
	e, ok := l.entries[ip]
	if !ok {
		return true
	}
	if time.Now().Before(e.blockedTo) {
		return false
	}
	return true
}

func (l *loginLimiter) fail(ip string) {
	if ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[ip]
	if !ok || now.Sub(e.firstAt) > l.window {
		l.entries[ip] = &limiterEntry{count: 1, firstAt: now}
		return
	}
	e.count++
	if e.count >= l.max {
		e.blockedTo = now.Add(l.window)
	}
}

func (l *loginLimiter) reset(ip string) {
	if ip == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

func (l *loginLimiter) gc() {
	now := time.Now()
	for k, e := range l.entries {
		if now.After(e.blockedTo) && now.Sub(e.firstAt) > l.window {
			delete(l.entries, k)
		}
	}
}
