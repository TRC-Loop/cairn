// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
)

type UsersHandler struct {
	q        *store.Queries
	sessions *auth.SessionService
	logger   *slog.Logger
}

func NewUsersHandler(q *store.Queries, sessions *auth.SessionService, logger *slog.Logger) *UsersHandler {
	return &UsersHandler{q: q, sessions: sessions, logger: logger}
}

type userFullResponse struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	TotpEnabled bool   `json:"totp_enabled"`
}

func toUserFull(u store.User) userFullResponse {
	return userFullResponse{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		CreatedAt:   u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   u.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		TotpEnabled: u.TotpEnabled,
	}
}

func (h *UsersHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.q.ListUsers(r.Context())
	if err != nil {
		h.logger.Error("list users failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]userFullResponse, 0, len(users))
	for _, u := range users {
		out = append(out, toUserFull(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out, "total": len(out)})
}

func (h *UsersHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	u, err := h.q.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserFull(u)})
}

type createUserRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

func (h *UsersHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Role = strings.TrimSpace(req.Role)

	fields := map[string]string{}
	switch {
	case req.Username == "":
		fields["username"] = FieldRequired
	case len(req.Username) < 3:
		fields["username"] = FieldTooShort
	case len(req.Username) > 64:
		fields["username"] = FieldTooLong
	case !usernameRe.MatchString(req.Username):
		fields["username"] = FieldInvalidFormat
	}
	if !isValidEmail(req.Email) {
		fields["email"] = FieldInvalidFormat
	}
	switch {
	case req.DisplayName == "":
		fields["display_name"] = FieldRequired
	case len(req.DisplayName) > 100:
		fields["display_name"] = FieldTooLong
	}
	switch {
	case req.Password == "":
		fields["password"] = FieldRequired
	case len(req.Password) < 12:
		fields["password"] = FieldTooShort
	case len(req.Password) > 512:
		fields["password"] = FieldTooLong
	}
	if !isValidRole(req.Role) {
		fields["role"] = FieldInvalidValue
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	if _, err := h.q.GetUserByUsername(r.Context(), req.Username); err == nil {
		writeError(w, http.StatusConflict, CodeConflict, "username already taken", map[string]string{"username": "duplicate"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		h.logger.Error("username lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if _, err := h.q.GetUserByEmail(r.Context(), req.Email); err == nil {
		writeError(w, http.StatusConflict, CodeConflict, "email already taken", map[string]string{"email": "duplicate"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		h.logger.Error("email lookup failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	hash, err := auth.Hash(req.Password)
	if err != nil {
		h.logger.Error("hash failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	u, err := h.q.CreateUser(r.Context(), store.CreateUserParams{
		Username:     req.Username,
		Email:        req.Email,
		DisplayName:  req.DisplayName,
		PasswordHash: hash,
		Role:         req.Role,
	})
	if err != nil {
		h.logger.Error("create user failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("user created", "user_id", u.ID, "role", u.Role)
	writeJSON(w, http.StatusCreated, map[string]any{"user": toUserFull(u)})
}

type updateUserRequest struct {
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
}

func (h *UsersHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	isSelf := actor.ID == id
	isAdmin := auth.Role(actor.Role) == auth.RoleAdmin
	if !isSelf && !isAdmin {
		writeError(w, http.StatusForbidden, CodeForbidden, "forbidden", nil)
		return
	}

	target, err := h.q.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	email := target.Email
	displayName := target.DisplayName
	role := target.Role
	fields := map[string]string{}

	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		if !isValidEmail(v) {
			fields["email"] = FieldInvalidFormat
		} else {
			email = v
		}
	}
	if req.DisplayName != nil {
		v := strings.TrimSpace(*req.DisplayName)
		switch {
		case v == "":
			fields["display_name"] = FieldRequired
		case len(v) > 100:
			fields["display_name"] = FieldTooLong
		default:
			displayName = v
		}
	}
	if req.Role != nil {
		v := strings.TrimSpace(*req.Role)
		switch {
		case isSelf && !isAdmin:
			fields["role"] = FieldImmutable
		case isSelf && isAdmin && v != target.Role:
			fields["role"] = FieldImmutable
		case !isValidRole(v):
			fields["role"] = FieldInvalidValue
		default:
			role = v
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	if email != target.Email {
		if existing, err := h.q.GetUserByEmail(r.Context(), email); err == nil && existing.ID != target.ID {
			writeError(w, http.StatusConflict, CodeConflict, "email already taken", map[string]string{"email": "duplicate"})
			return
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("email lookup failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
	}

	if err := h.q.UpdateUserMetadata(r.Context(), store.UpdateUserMetadataParams{
		Email: email, DisplayName: displayName, Role: role, ID: target.ID,
	}); err != nil {
		h.logger.Error("update user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	updated, err := h.q.GetUserByID(r.Context(), target.ID)
	if err != nil {
		h.logger.Error("get updated user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("user updated", "user_id", updated.ID, "actor_id", actor.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": toUserFull(updated)})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ForceLogout     bool   `json:"force_logout"`
}

func (h *UsersHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	isSelf := actor.ID == id
	isAdmin := auth.Role(actor.Role) == auth.RoleAdmin
	if !isSelf && !isAdmin {
		writeError(w, http.StatusForbidden, CodeForbidden, "forbidden", nil)
		return
	}

	target, err := h.q.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	fields := map[string]string{}
	switch {
	case req.NewPassword == "":
		fields["new_password"] = FieldRequired
	case len(req.NewPassword) < 12:
		fields["new_password"] = FieldTooShort
	case len(req.NewPassword) > 512:
		fields["new_password"] = FieldTooLong
	}
	if isSelf && req.CurrentPassword == "" {
		fields["current_password"] = FieldRequired
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	if isSelf {
		ok, err := auth.Verify(req.CurrentPassword, target.PasswordHash)
		if err != nil || !ok {
			writeError(w, http.StatusUnauthorized, CodeUnauthorized, "current password incorrect", nil)
			return
		}
	}

	hash, err := auth.Hash(req.NewPassword)
	if err != nil {
		h.logger.Error("hash failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.q.UpdateUserPassword(r.Context(), store.UpdateUserPasswordParams{PasswordHash: hash, ID: target.ID}); err != nil {
		h.logger.Error("update password failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	if isSelf {
		sess, _ := auth.SessionFromContext(r.Context())
		if err := h.q.DeleteSessionsForUserExcept(r.Context(), store.DeleteSessionsForUserExceptParams{
			UserID: target.ID, ID: sess.ID,
		}); err != nil {
			h.logger.Warn("revoke other sessions failed", "user_id", target.ID, "err", err)
		}
	} else if req.ForceLogout {
		if err := h.sessions.RevokeAllForUser(r.Context(), target.ID); err != nil {
			h.logger.Warn("revoke all sessions failed", "user_id", target.ID, "err", err)
		}
	}

	h.logger.Info("password changed", "user_id", target.ID, "actor_id", actor.ID, "self", isSelf)
	w.WriteHeader(http.StatusNoContent)
}

func (h *UsersHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	actor, _ := auth.UserFromContext(r.Context())
	if actor.ID == id {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "cannot delete your own account", nil)
		return
	}
	target, err := h.q.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if target.Role == string(auth.RoleAdmin) {
		count, err := h.q.CountUsersByRole(r.Context(), string(auth.RoleAdmin))
		if err != nil {
			h.logger.Error("count admins failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "cannot delete the last admin", nil)
			return
		}
	}
	if err := h.q.DeleteUser(r.Context(), target.ID); err != nil {
		h.logger.Error("delete user failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.sessions.RevokeAllForUser(r.Context(), target.ID); err != nil {
		h.logger.Warn("revoke sessions on delete failed", "user_id", target.ID, "err", err)
	}
	h.logger.Info("user deleted", "user_id", target.ID, "actor_id", actor.ID)
	w.WriteHeader(http.StatusNoContent)
}

func isValidRole(s string) bool {
	switch auth.Role(s) {
	case auth.RoleAdmin, auth.RoleEditor, auth.RoleViewer:
		return true
	}
	return false
}

// userIDFromURL exists so middleware can determine target id without re-parsing twice.
func userIDFromURL(r *http.Request) string {
	return chi.URLParam(r, "id")
}
