// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type RetentionSettingsHandler struct {
	q      *store.Queries
	logger *slog.Logger
}

func NewRetentionSettingsHandler(q *store.Queries, logger *slog.Logger) *RetentionSettingsHandler {
	return &RetentionSettingsHandler{q: q, logger: logger}
}

type retentionSettingsResponse struct {
	RawDays          int64  `json:"raw_days"`
	HourlyDays       int64  `json:"hourly_days"`
	DailyDays        int64  `json:"daily_days"`
	KeepDailyForever bool   `json:"keep_daily_forever"`
	UpdatedAt        string `json:"updated_at"`
}

func toRetentionSettings(s store.RetentionSetting) retentionSettingsResponse {
	return retentionSettingsResponse{
		RawDays:          s.RawDays,
		HourlyDays:       s.HourlyDays,
		DailyDays:        s.DailyDays,
		KeepDailyForever: s.KeepDailyForever,
		UpdatedAt:        s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *RetentionSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.q.GetRetentionSettings(r.Context())
	if err != nil {
		h.logger.Error("get retention settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": toRetentionSettings(s)})
}

type updateRetentionRequest struct {
	RawDays          *int64 `json:"raw_days"`
	HourlyDays       *int64 `json:"hourly_days"`
	DailyDays        *int64 `json:"daily_days"`
	KeepDailyForever *bool  `json:"keep_daily_forever"`
}

func (h *RetentionSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	current, err := h.q.GetRetentionSettings(r.Context())
	if err != nil {
		h.logger.Error("get retention settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req updateRetentionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	raw := current.RawDays
	hourly := current.HourlyDays
	daily := current.DailyDays
	keep := current.KeepDailyForever

	fields := map[string]string{}
	if req.RawDays != nil {
		if *req.RawDays < 1 || *req.RawDays > 90 {
			fields["raw_days"] = FieldOutOfRange
		} else {
			raw = *req.RawDays
		}
	}
	if req.HourlyDays != nil {
		if *req.HourlyDays < 1 || *req.HourlyDays > 3650 {
			fields["hourly_days"] = FieldOutOfRange
		} else {
			hourly = *req.HourlyDays
		}
	}
	if req.DailyDays != nil {
		if *req.DailyDays < 1 || *req.DailyDays > 3650 {
			fields["daily_days"] = FieldOutOfRange
		} else {
			daily = *req.DailyDays
		}
	}
	if req.KeepDailyForever != nil {
		keep = *req.KeepDailyForever
	}

	if len(fields) == 0 {
		if hourly < raw {
			fields["hourly_days"] = FieldOutOfRange
		}
		if daily < hourly {
			fields["daily_days"] = FieldOutOfRange
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	if err := h.q.UpdateRetentionSettings(r.Context(), store.UpdateRetentionSettingsParams{
		RawDays: raw, HourlyDays: hourly, DailyDays: daily, KeepDailyForever: keep,
	}); err != nil {
		h.logger.Error("update retention settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	updated, err := h.q.GetRetentionSettings(r.Context())
	if err != nil {
		h.logger.Error("reload retention settings failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("retention settings updated", "raw", raw, "hourly", hourly, "daily", daily, "keep_daily", keep)
	writeJSON(w, http.StatusOK, map[string]any{"settings": toRetentionSettings(updated)})
}
