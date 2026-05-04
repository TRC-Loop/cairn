// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type SystemSettingsHandler struct {
	q      *store.Queries
	logger *slog.Logger
}

func NewSystemSettingsHandler(q *store.Queries, logger *slog.Logger) *SystemSettingsHandler {
	return &SystemSettingsHandler{q: q, logger: logger}
}

type systemSettingsResponse struct {
	IncidentIDFormat            string `json:"incident_id_format"`
	IncidentReopenWindowSeconds int64  `json:"incident_reopen_window_seconds"`
	IncidentReopenMode          string `json:"incident_reopen_mode"`
	UpdatedAt                   string `json:"updated_at"`
}

func toSystemSettings(s store.SystemSetting) systemSettingsResponse {
	return systemSettingsResponse{
		IncidentIDFormat:            s.IncidentIDFormat,
		IncidentReopenWindowSeconds: s.IncidentReopenWindowSeconds,
		IncidentReopenMode:          s.IncidentReopenMode,
		UpdatedAt:                   s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *SystemSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.q.GetSystemSettings(r.Context())
	if err != nil {
		h.logger.Error("get system settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": toSystemSettings(s)})
}

type updateSystemSettingsRequest struct {
	IncidentIDFormat            *string `json:"incident_id_format"`
	IncidentReopenWindowSeconds *int64  `json:"incident_reopen_window_seconds"`
	IncidentReopenMode          *string `json:"incident_reopen_mode"`
}

func (h *SystemSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	current, err := h.q.GetSystemSettings(r.Context())
	if err != nil {
		h.logger.Error("get system settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req updateSystemSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	format := current.IncidentIDFormat
	window := current.IncidentReopenWindowSeconds
	mode := current.IncidentReopenMode

	fields := map[string]string{}
	if req.IncidentIDFormat != nil {
		v := strings.TrimSpace(*req.IncidentIDFormat)
		switch {
		case v == "":
			fields["incident_id_format"] = FieldRequired
		case len(v) > 200:
			fields["incident_id_format"] = FieldTooLong
		default:
			format = v
		}
	}
	if req.IncidentReopenWindowSeconds != nil {
		v := *req.IncidentReopenWindowSeconds
		if v < 0 || v > 604800 {
			fields["incident_reopen_window_seconds"] = FieldOutOfRange
		} else {
			window = v
		}
	}
	if req.IncidentReopenMode != nil {
		v := strings.TrimSpace(*req.IncidentReopenMode)
		if !validReopenMode(v) {
			fields["incident_reopen_mode"] = FieldInvalidValue
		} else {
			mode = v
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	if !strings.Contains(format, "{id}") {
		h.logger.Warn("incident id format missing {id}", "format", format, "hint", "duplicates possible")
	}

	if err := h.q.UpdateSystemSettings(r.Context(), store.UpdateSystemSettingsParams{
		IncidentIDFormat:            format,
		IncidentReopenWindowSeconds: window,
		IncidentReopenMode:          mode,
	}); err != nil {
		h.logger.Error("update system settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	updated, err := h.q.GetSystemSettings(r.Context())
	if err != nil {
		h.logger.Error("reload system settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("system settings updated", "format", format, "window", window, "mode", mode)
	writeJSON(w, http.StatusOK, map[string]any{"settings": toSystemSettings(updated)})
}
