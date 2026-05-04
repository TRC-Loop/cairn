// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/skip2/go-qrcode"
)

const (
	twoFAChallengeTTL = 5 * time.Minute
	twoFAMaxAttempts  = 3
	recoveryCodeCount = 10
)

type TwoFAHandler struct {
	q          *store.Queries
	sessions   *auth.SessionService
	secretBox  *crypto.SecretBox
	logger     *slog.Logger
	behindTLS  bool
	challenges *challengeStore
}

func NewTwoFAHandler(q *store.Queries, sessions *auth.SessionService, secretBox *crypto.SecretBox, logger *slog.Logger, behindTLS bool) *TwoFAHandler {
	cs := newChallengeStore()
	go cs.runCleanup(context.Background())
	return &TwoFAHandler{q: q, sessions: sessions, secretBox: secretBox, logger: logger, behindTLS: behindTLS, challenges: cs}
}

// ------------------------------------------------------------------
// Enrollment endpoints (require auth).
// ------------------------------------------------------------------

type setupResponse struct {
	Secret        string `json:"secret"`
	OtpauthURL    string `json:"otpauth_url"`
	QRCodeDataURL string `json:"qr_code_data_url"`
}

func (h *TwoFAHandler) Setup(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user.TotpEnabled {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "2fa already enabled", nil)
		return
	}
	secret, otpauthURL, err := auth.GenerateTOTPSecret(user.Username)
	if err != nil {
		h.logger.Error("totp generate failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	enc, err := h.secretBox.EncryptString(secret)
	if err != nil {
		h.logger.Error("encrypt totp secret failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.SetUserTOTPSecret(r.Context(), store.SetUserTOTPSecretParams{
		TotpSecretEnc: sql.NullString{String: enc, Valid: true}, ID: user.ID,
	}); err != nil {
		h.logger.Error("store totp secret failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, 220)
	if err != nil {
		h.logger.Error("qr encode failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, setupResponse{
		Secret:        secret,
		OtpauthURL:    otpauthURL,
		QRCodeDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	})
}

type confirmRequest struct {
	Code string `json:"code"`
}

type recoveryCodesResponse struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

func (h *TwoFAHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user.TotpEnabled {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "2fa already enabled", nil)
		return
	}
	if !user.TotpSecretEnc.Valid {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "no setup in progress", nil)
		return
	}
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "missing code", map[string]string{"code": FieldRequired})
		return
	}
	secret, err := h.secretBox.DecryptString(user.TotpSecretEnc.String)
	if err != nil {
		h.logger.Error("decrypt totp secret failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if !auth.VerifyTOTP(secret, req.Code) {
		writeError(w, http.StatusBadRequest, CodeUnauthorized, "invalid code", map[string]string{"code": FieldInvalidValue})
		return
	}
	plaintext, hashes, err := auth.GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		h.logger.Error("recovery codes failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.EnableUserTOTP(r.Context(), user.ID); err != nil {
		h.logger.Error("enable totp failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.DeleteRecoveryCodesForUser(r.Context(), user.ID); err != nil {
		h.logger.Warn("clear old recovery codes failed", "err", err, "user_id", user.ID)
	}
	for _, hh := range hashes {
		if _, err := h.q.CreateRecoveryCode(r.Context(), store.CreateRecoveryCodeParams{UserID: user.ID, CodeHash: hh}); err != nil {
			h.logger.Error("create recovery code failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
	}
	h.logger.Info("2fa enabled", "user_id", user.ID)
	writeJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: plaintext})
}

type disableRequest struct {
	CurrentPassword string `json:"current_password"`
	Code            string `json:"code"`
}

func (h *TwoFAHandler) Disable(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if !user.TotpEnabled {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "2fa not enabled", nil)
		return
	}
	var req disableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if !h.verifyPasswordAndCode(r.Context(), w, user, req.CurrentPassword, req.Code) {
		return
	}
	if err := h.q.DisableUserTOTP(r.Context(), user.ID); err != nil {
		h.logger.Error("disable totp failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.DeleteRecoveryCodesForUser(r.Context(), user.ID); err != nil {
		h.logger.Warn("delete recovery codes failed", "err", err, "user_id", user.ID)
	}
	h.logger.Info("2fa disabled", "user_id", user.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *TwoFAHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if !user.TotpEnabled {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "2fa not enabled", nil)
		return
	}
	var req disableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if !h.verifyPasswordAndCode(r.Context(), w, user, req.CurrentPassword, req.Code) {
		return
	}
	plaintext, hashes, err := auth.GenerateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		h.logger.Error("recovery codes failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.DeleteRecoveryCodesForUser(r.Context(), user.ID); err != nil {
		h.logger.Error("delete recovery codes failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	for _, hh := range hashes {
		if _, err := h.q.CreateRecoveryCode(r.Context(), store.CreateRecoveryCodeParams{UserID: user.ID, CodeHash: hh}); err != nil {
			h.logger.Error("create recovery code failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
	}
	h.logger.Info("2fa recovery codes regenerated", "user_id", user.ID)
	writeJSON(w, http.StatusOK, recoveryCodesResponse{RecoveryCodes: plaintext})
}

type statusResponse struct {
	Enabled                bool    `json:"enabled"`
	EnrolledAt             *string `json:"enrolled_at"`
	RecoveryCodesRemaining int64   `json:"recovery_codes_remaining"`
}

func (h *TwoFAHandler) Status(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	resp := statusResponse{Enabled: user.TotpEnabled}
	if user.TotpEnrolledAt.Valid {
		s := user.TotpEnrolledAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		resp.EnrolledAt = &s
	}
	if user.TotpEnabled {
		n, err := h.q.CountUnusedRecoveryCodes(r.Context(), user.ID)
		if err != nil {
			h.logger.Error("count recovery codes failed", "err", err)
		}
		resp.RecoveryCodesRemaining = n
	}
	writeJSON(w, http.StatusOK, resp)
}

// ------------------------------------------------------------------
// Helpers
// ------------------------------------------------------------------

// verifyPasswordAndCode reads the freshest user record (TotpSecretEnc may have changed)
// and validates both factors. Writes its own error response on failure.
func (h *TwoFAHandler) verifyPasswordAndCode(ctx context.Context, w http.ResponseWriter, user store.User, password, code string) bool {
	if password == "" || code == "" {
		fields := map[string]string{}
		if password == "" {
			fields["current_password"] = FieldRequired
		}
		if code == "" {
			fields["code"] = FieldRequired
		}
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return false
	}
	fresh, err := h.q.GetUserByID(ctx, user.ID)
	if err != nil {
		h.logger.Error("get user failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return false
	}
	ok, err := auth.Verify(password, fresh.PasswordHash)
	if err != nil || !ok {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid credentials", nil)
		return false
	}
	if !h.verifyCode(ctx, fresh, code) {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid code", nil)
		return false
	}
	return true
}

// verifyCode accepts either a TOTP digit code or a recovery code. On a
// successful recovery code match, the code is marked used.
func (h *TwoFAHandler) verifyCode(ctx context.Context, user store.User, code string) bool {
	if !user.TotpSecretEnc.Valid {
		return false
	}
	if auth.LooksLikeRecoveryCode(code) {
		return h.consumeRecoveryCode(ctx, user.ID, code)
	}
	secret, err := h.secretBox.DecryptString(user.TotpSecretEnc.String)
	if err != nil {
		h.logger.Error("decrypt totp secret failed", "err", err)
		return false
	}
	if auth.VerifyTOTP(secret, code) {
		return true
	}
	// Allow fallback: pasted recovery code without dashes? Try recovery list anyway.
	return h.consumeRecoveryCode(ctx, user.ID, code)
}

func (h *TwoFAHandler) consumeRecoveryCode(ctx context.Context, userID int64, code string) bool {
	codes, err := h.q.ListUnusedRecoveryCodesForUser(ctx, userID)
	if err != nil {
		h.logger.Error("list recovery codes failed", "err", err)
		return false
	}
	for _, rc := range codes {
		if auth.VerifyRecoveryCode(code, rc.CodeHash) {
			if err := h.q.MarkRecoveryCodeUsed(ctx, rc.ID); err != nil {
				h.logger.Error("mark recovery code used failed", "err", err)
				return false
			}
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------
// Login challenge flow
// ------------------------------------------------------------------

type challengeState struct {
	userID    int64
	expiresAt time.Time
	attempts  int
}

type challengeStore struct {
	mu      sync.Mutex
	entries map[string]*challengeState
}

func newChallengeStore() *challengeStore {
	return &challengeStore{entries: map[string]*challengeState{}}
}

func (c *challengeStore) issue(userID int64) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(buf)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[tok] = &challengeState{userID: userID, expiresAt: time.Now().Add(twoFAChallengeTTL)}
	return tok, nil
}

func (c *challengeStore) get(tok string) (*challengeState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[tok]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.entries, tok)
		return nil, false
	}
	return e, true
}

func (c *challengeStore) consume(tok string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, tok)
}

func (c *challengeStore) recordFailure(tok string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[tok]
	if !ok {
		return true
	}
	e.attempts++
	if e.attempts >= twoFAMaxAttempts {
		delete(c.entries, tok)
		return true
	}
	return false
}

func (c *challengeStore) runCleanup(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

// ChallengeIssue is called by AuthHandler when login succeeds for a 2FA-enabled user.
// Returns the opaque challenge token.
func (h *TwoFAHandler) ChallengeIssue(userID int64) (string, error) {
	return h.challenges.issue(userID)
}

type loginVerifyRequest struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
}

// LoginVerify completes the 2FA challenge. Sets the session cookie on success.
// The login limiter is checked by the parent AuthHandler; we still apply per-token strikes.
func (h *TwoFAHandler) LoginVerify(w http.ResponseWriter, r *http.Request) {
	var req loginVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	req.ChallengeToken = strings.TrimSpace(req.ChallengeToken)
	req.Code = strings.TrimSpace(req.Code)
	if req.ChallengeToken == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "missing fields", map[string]string{"challenge_token": FieldRequired})
		return
	}
	state, ok := h.challenges.get(req.ChallengeToken)
	if !ok {
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "challenge expired", nil)
		return
	}
	user, err := h.q.GetUserByID(r.Context(), state.userID)
	if err != nil {
		h.logger.Error("get user failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if !user.TotpEnabled {
		// Defensive: 2FA was disabled mid-flight.
		h.challenges.consume(req.ChallengeToken)
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "challenge expired", nil)
		return
	}
	if !h.verifyCode(r.Context(), user, req.Code) {
		invalidated := h.challenges.recordFailure(req.ChallengeToken)
		if invalidated {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "too many attempts", nil)
			return
		}
		writeError(w, http.StatusUnauthorized, CodeUnauthorized, "invalid code", nil)
		return
	}
	h.challenges.consume(req.ChallengeToken)
	sid, err := h.sessions.Create(r.Context(), user.ID, r.UserAgent(), clientIP(r))
	if err != nil {
		h.logger.Error("create session failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   h.behindTLS,
		SameSite: http.SameSiteLaxMode,
	})
	h.logger.Info("login 2fa success", "user_id", user.ID, "ip", clientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserResponse(user)})
}

// ------------------------------------------------------------------
// Admin reset
// ------------------------------------------------------------------

func (h *TwoFAHandler) AdminReset(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	target, err := h.q.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get user failed", "err", err, "id", id)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.DisableUserTOTP(r.Context(), target.ID); err != nil {
		h.logger.Error("disable totp failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.DeleteRecoveryCodesForUser(r.Context(), target.ID); err != nil {
		h.logger.Warn("delete recovery codes failed", "err", err, "user_id", target.ID)
	}
	actor, _ := auth.UserFromContext(r.Context())
	h.logger.Info("admin reset 2fa", "actor_username", actor.Username, "actor_id", actor.ID, "target_username", target.Username, "target_id", target.ID)
	w.WriteHeader(http.StatusNoContent)
}

