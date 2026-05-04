// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
)

type SetupHandler struct {
	q         *store.Queries
	db        *sql.DB
	sessions  *auth.SessionService
	logger    *slog.Logger
	behindTLS bool
}

func NewSetupHandler(q *store.Queries, db *sql.DB, sessions *auth.SessionService, logger *slog.Logger, behindTLS bool) *SetupHandler {
	return &SetupHandler{q: q, db: db, sessions: sessions, logger: logger, behindTLS: behindTLS}
}

type setupCompleteRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	count, err := h.q.CountUsers(r.Context())
	if err != nil {
		h.logger.Error("setup status count failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"setup_complete": count > 0})
}

func (h *SetupHandler) Complete(w http.ResponseWriter, r *http.Request) {
	var req setupCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	if fields := validateSetup(&req); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	hash, err := auth.Hash(req.Password)
	if err != nil {
		h.logger.Error("setup hash failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		h.logger.Error("setup begin tx failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	defer tx.Rollback()

	qtx := h.q.WithTx(tx)

	count, err := qtx.CountUsers(r.Context())
	if err != nil {
		h.logger.Error("setup recount failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if count > 0 {
		writeError(w, http.StatusConflict, CodeConflict, "setup already completed", nil)
		return
	}

	user, err := qtx.CreateUser(r.Context(), store.CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: hash,
		Role:         "admin",
	})
	if err != nil {
		h.logger.Error("setup create user failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	if err := tx.Commit(); err != nil {
		h.logger.Error("setup commit failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	sessionID, err := h.sessions.Create(r.Context(), user.ID, r.UserAgent(), clientIP(r))
	if err != nil {
		h.logger.Error("setup create session failed", "err", err)
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

	h.logger.Info("setup completed", "user_id", user.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": toUserResponse(user),
	})
}

func validateSetup(req *setupCompleteRequest) map[string]string {
	f := map[string]string{}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	switch {
	case req.Username == "":
		f["username"] = FieldRequired
	case len(req.Username) < 3:
		f["username"] = FieldTooShort
	case len(req.Username) > 64:
		f["username"] = FieldTooLong
	case !usernameRe.MatchString(req.Username):
		f["username"] = FieldInvalidFormat
	}
	if !isValidEmail(req.Email) {
		f["email"] = FieldInvalidFormat
	}
	switch {
	case req.DisplayName == "":
		f["display_name"] = FieldRequired
	case len(req.DisplayName) > 100:
		f["display_name"] = FieldTooLong
	}
	switch {
	case req.Password == "":
		f["password"] = FieldRequired
	case len(req.Password) < 12:
		f["password"] = FieldTooShort
	case len(req.Password) > 512:
		f["password"] = FieldTooLong
	}
	return f
}

func isValidEmail(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.Count(s, "@") != 1 {
		return false
	}
	domain := s[at+1:]
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}
