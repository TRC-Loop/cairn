// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/check"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
)

type ChecksHandler struct {
	q           *store.Queries
	db          *sql.DB
	logger      *slog.Logger
	registry    *check.Registry
	incidentSvc check.IncidentService
}

func NewChecksHandler(q *store.Queries, db *sql.DB, logger *slog.Logger) *ChecksHandler {
	return &ChecksHandler{q: q, db: db, logger: logger}
}

func (h *ChecksHandler) SetRunner(registry *check.Registry, incidentSvc check.IncidentService) {
	h.registry = registry
	h.incidentSvc = incidentSvc
}

var validCheckTypes = map[string]struct{}{
	"http": {}, "tcp": {}, "icmp": {}, "dns": {}, "tls": {},
	"push": {}, "db_postgres": {}, "db_mysql": {}, "db_redis": {}, "grpc": {},
}

type checkResponse struct {
	ID                     int64          `json:"id"`
	Name                   string         `json:"name"`
	Type                   string         `json:"type"`
	Enabled                bool           `json:"enabled"`
	IntervalSeconds        int64          `json:"interval_seconds"`
	TimeoutSeconds         int64          `json:"timeout_seconds"`
	Retries                int64          `json:"retries"`
	FailureThreshold       int64          `json:"failure_threshold"`
	RecoveryThreshold      int64          `json:"recovery_threshold"`
	Config                 map[string]any `json:"config"`
	ComponentID            *int64         `json:"component_id"`
	NotificationChannelIDs []int64        `json:"notification_channel_ids"`
	LastStatus             string         `json:"last_status"`
	LastLatencyMs          *int64         `json:"last_latency_ms"`
	LastCheckedAt          *time.Time     `json:"last_checked_at"`
	ConsecutiveFailures    int64          `json:"consecutive_failures"`
	ConsecutiveSuccesses   int64          `json:"consecutive_successes"`
	PushToken              *string        `json:"push_token,omitempty"`
	ReopenWindowSeconds    *int64         `json:"reopen_window_seconds"`
	ReopenMode             *string        `json:"reopen_mode"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

func toCheckResponse(c store.Check, includePushToken bool) checkResponse {
	out := checkResponse{
		ID:                     c.ID,
		Name:                   c.Name,
		Type:                   c.Type,
		Enabled:                c.Enabled,
		IntervalSeconds:        c.IntervalSeconds,
		TimeoutSeconds:         c.TimeoutSeconds,
		Retries:                c.Retries,
		FailureThreshold:       c.FailureThreshold,
		RecoveryThreshold:      c.RecoveryThreshold,
		LastStatus:             c.LastStatus,
		ConsecutiveFailures:    c.ConsecutiveFailures,
		ConsecutiveSuccesses:   c.ConsecutiveSuccesses,
		CreatedAt:              c.CreatedAt,
		UpdatedAt:              c.UpdatedAt,
		Config:                 map[string]any{},
		NotificationChannelIDs: []int64{},
	}
	if c.ConfigJson != "" {
		_ = json.Unmarshal([]byte(c.ConfigJson), &out.Config)
	}
	if c.ComponentID.Valid {
		v := c.ComponentID.Int64
		out.ComponentID = &v
	}
	if c.LastLatencyMs.Valid {
		v := c.LastLatencyMs.Int64
		out.LastLatencyMs = &v
	}
	if c.LastCheckedAt.Valid {
		v := c.LastCheckedAt.Time
		out.LastCheckedAt = &v
	}
	if includePushToken && c.PushToken.Valid {
		v := c.PushToken.String
		out.PushToken = &v
	}
	if c.ReopenWindowSeconds.Valid {
		v := c.ReopenWindowSeconds.Int64
		out.ReopenWindowSeconds = &v
	}
	if c.ReopenMode.Valid {
		v := c.ReopenMode.String
		out.ReopenMode = &v
	}
	return out
}

type checkWriteRequest struct {
	Name                   *string         `json:"name"`
	Type                   *string         `json:"type"`
	Enabled                *bool           `json:"enabled"`
	IntervalSeconds        *int64          `json:"interval_seconds"`
	TimeoutSeconds         *int64          `json:"timeout_seconds"`
	Retries                *int64          `json:"retries"`
	FailureThreshold       *int64          `json:"failure_threshold"`
	RecoveryThreshold      *int64          `json:"recovery_threshold"`
	Config                 json.RawMessage `json:"config"`
	ComponentID            *int64          `json:"component_id"`
	NotificationChannelIDs *[]int64        `json:"notification_channel_ids"`
	ReopenWindowSeconds    jsonNullable[int64]  `json:"reopen_window_seconds"`
	ReopenMode             jsonNullable[string] `json:"reopen_mode"`
}

// jsonNullable distinguishes "field missing" from "field set to null" for PATCH semantics.
type jsonNullable[T any] struct {
	Set   bool
	Valid bool
	Value T
}

func (n *jsonNullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	if string(data) == "null" {
		n.Valid = false
		return nil
	}
	n.Valid = true
	return json.Unmarshal(data, &n.Value)
}

func (h *ChecksHandler) List(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	includeToken := auth.HasAtLeast(auth.Role(user.Role), auth.RoleEditor)

	rows, err := h.q.ListChecks(r.Context())
	if err != nil {
		h.logger.Error("list checks failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]checkResponse, 0, len(rows))
	for _, c := range rows {
		resp := toCheckResponse(c, includeToken)
		if ids, _ := h.q.ListChannelsForCheck(r.Context(), c.ID); ids != nil {
			resp.NotificationChannelIDs = ids
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": out})
}

func (h *ChecksHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	user, _ := auth.UserFromContext(r.Context())
	includeToken := auth.HasAtLeast(auth.Role(user.Role), auth.RoleEditor)

	c, err := h.q.GetCheck(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get check failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	resp := toCheckResponse(c, includeToken)
	if ids, _ := h.q.ListChannelsForCheck(r.Context(), c.ID); ids != nil {
		resp.NotificationChannelIDs = ids
	}
	writeJSON(w, http.StatusOK, map[string]any{"check": resp})
}

// setChannelsForCheck atomically replaces the channel set associated with a
// check. Use during Create/Update.
func (h *ChecksHandler) setChannelsForCheck(ctx context.Context, checkID int64, ids []int64) error {
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	qtx := h.q.WithTx(tx)
	if err := qtx.ClearChannelsForCheck(ctx, checkID); err != nil {
		return err
	}
	for _, id := range ids {
		if err := qtx.LinkCheckToChannel(ctx, store.LinkCheckToChannelParams{
			CheckID: checkID, ChannelID: id,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (h *ChecksHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req checkWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	if req.Name == nil || req.Type == nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"name": FieldRequired, "type": FieldRequired})
		return
	}

	params := store.CreateCheckParams{
		Name:              *req.Name,
		Type:              *req.Type,
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		Retries:           0,
		FailureThreshold:  3,
		RecoveryThreshold: 1,
		ConfigJson:        "{}",
	}
	if req.Enabled != nil {
		params.Enabled = *req.Enabled
	}
	if req.IntervalSeconds != nil {
		params.IntervalSeconds = *req.IntervalSeconds
	}
	if req.TimeoutSeconds != nil {
		params.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.Retries != nil {
		params.Retries = *req.Retries
	}
	if req.FailureThreshold != nil {
		params.FailureThreshold = *req.FailureThreshold
	}
	if req.RecoveryThreshold != nil {
		params.RecoveryThreshold = *req.RecoveryThreshold
	}
	if req.ComponentID != nil {
		params.ComponentID = sql.NullInt64{Int64: *req.ComponentID, Valid: true}
	}
	if req.ReopenWindowSeconds.Set && req.ReopenWindowSeconds.Valid {
		params.ReopenWindowSeconds = sql.NullInt64{Int64: req.ReopenWindowSeconds.Value, Valid: true}
	}
	if req.ReopenMode.Set && req.ReopenMode.Valid {
		params.ReopenMode = sql.NullString{String: req.ReopenMode.Value, Valid: true}
	}

	if params.Type == "push" {
		params.ConfigJson = "{}"
	} else if len(req.Config) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(req.Config, &obj); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"config": FieldInvalidFormat})
			return
		}
		b, _ := json.Marshal(obj)
		params.ConfigJson = string(b)
	}

	if fields := validateCheckParams(params.Name, params.Type, params.IntervalSeconds, params.TimeoutSeconds, params.Retries, params.FailureThreshold, params.RecoveryThreshold); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	if params.ReopenMode.Valid && !validReopenMode(params.ReopenMode.String) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"reopen_mode": FieldInvalidValue})
		return
	}
	if params.ReopenWindowSeconds.Valid && (params.ReopenWindowSeconds.Int64 < 0 || params.ReopenWindowSeconds.Int64 > 604800) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"reopen_window_seconds": FieldOutOfRange})
		return
	}

	c, err := h.q.CreateCheck(r.Context(), params)
	if err != nil {
		h.logger.Error("create check failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	if c.Type == "push" {
		tok, err := check.GenerateToken()
		if err != nil {
			h.logger.Error("generate push token failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		if err := h.q.SetCheckPushToken(r.Context(), store.SetCheckPushTokenParams{
			PushToken: sql.NullString{String: tok, Valid: true},
			ID:        c.ID,
		}); err != nil {
			h.logger.Error("set push token failed", "id", c.ID, "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		c.PushToken = sql.NullString{String: tok, Valid: true}
	}

	if req.NotificationChannelIDs != nil {
		if err := h.setChannelsForCheck(r.Context(), c.ID, *req.NotificationChannelIDs); err != nil {
			h.logger.Error("set channels for check failed", "id", c.ID, "err", err)
		}
	}

	h.logger.Info("check created", "id", c.ID, "type", c.Type)
	h.runOnceAfterCreate(c)
	resp := toCheckResponse(c, true)
	if ids, _ := h.q.ListChannelsForCheck(r.Context(), c.ID); ids != nil {
		resp.NotificationChannelIDs = ids
	}
	writeJSON(w, http.StatusCreated, map[string]any{"check": resp})
}

func (h *ChecksHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	existing, err := h.q.GetCheck(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get check failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req checkWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if req.Type != nil && *req.Type != existing.Type {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"type": FieldImmutable})
		return
	}

	params := store.UpdateCheckParams{
		ID:                existing.ID,
		Name:              existing.Name,
		Type:              existing.Type,
		Enabled:           existing.Enabled,
		IntervalSeconds:   existing.IntervalSeconds,
		TimeoutSeconds:    existing.TimeoutSeconds,
		Retries:           existing.Retries,
		FailureThreshold:  existing.FailureThreshold,
		RecoveryThreshold: existing.RecoveryThreshold,
		ConfigJson:        existing.ConfigJson,
		ComponentID:       existing.ComponentID,
		ReopenWindowSeconds: existing.ReopenWindowSeconds,
		ReopenMode:          existing.ReopenMode,
	}
	if req.Name != nil {
		params.Name = *req.Name
	}
	if req.Enabled != nil {
		params.Enabled = *req.Enabled
	}
	if req.IntervalSeconds != nil {
		params.IntervalSeconds = *req.IntervalSeconds
	}
	if req.TimeoutSeconds != nil {
		params.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.Retries != nil {
		params.Retries = *req.Retries
	}
	if req.FailureThreshold != nil {
		params.FailureThreshold = *req.FailureThreshold
	}
	if req.RecoveryThreshold != nil {
		params.RecoveryThreshold = *req.RecoveryThreshold
	}
	if len(req.Config) > 0 {
		var obj map[string]any
		if err := json.Unmarshal(req.Config, &obj); err != nil {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"config": FieldInvalidFormat})
			return
		}
		b, _ := json.Marshal(obj)
		params.ConfigJson = string(b)
	}
	if req.ComponentID != nil {
		params.ComponentID = sql.NullInt64{Int64: *req.ComponentID, Valid: true}
	}
	if req.ReopenWindowSeconds.Set {
		if req.ReopenWindowSeconds.Valid {
			params.ReopenWindowSeconds = sql.NullInt64{Int64: req.ReopenWindowSeconds.Value, Valid: true}
		} else {
			params.ReopenWindowSeconds = sql.NullInt64{}
		}
	}
	if req.ReopenMode.Set {
		if req.ReopenMode.Valid {
			params.ReopenMode = sql.NullString{String: req.ReopenMode.Value, Valid: true}
		} else {
			params.ReopenMode = sql.NullString{}
		}
	}

	if fields := validateCheckParams(params.Name, params.Type, params.IntervalSeconds, params.TimeoutSeconds, params.Retries, params.FailureThreshold, params.RecoveryThreshold); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	if params.ReopenMode.Valid && !validReopenMode(params.ReopenMode.String) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"reopen_mode": FieldInvalidValue})
		return
	}
	if params.ReopenWindowSeconds.Valid && (params.ReopenWindowSeconds.Int64 < 0 || params.ReopenWindowSeconds.Int64 > 604800) {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"reopen_window_seconds": FieldOutOfRange})
		return
	}

	c, err := h.q.UpdateCheck(r.Context(), params)
	if err != nil {
		h.logger.Error("update check failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if req.NotificationChannelIDs != nil {
		if err := h.setChannelsForCheck(r.Context(), c.ID, *req.NotificationChannelIDs); err != nil {
			h.logger.Error("set channels for check failed", "id", c.ID, "err", err)
		}
	}

	h.logger.Info("check updated", "id", c.ID)
	resp := toCheckResponse(c, true)
	if ids, _ := h.q.ListChannelsForCheck(r.Context(), c.ID); ids != nil {
		resp.NotificationChannelIDs = ids
	}
	writeJSON(w, http.StatusOK, map[string]any{"check": resp})
}

func (h *ChecksHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.q.DeleteCheck(r.Context(), id); err != nil {
		h.logger.Error("delete check failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("check deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

type checkResultRow struct {
	CheckedAt    time.Time `json:"checked_at"`
	Status       string    `json:"status"`
	LatencyMs    *int64    `json:"latency_ms"`
	ErrorMessage *string   `json:"error_message"`
}

func (h *ChecksHandler) RecentResults(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	if limitStr := q.Get("limit"); limitStr != "" {
		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 {
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "invalid limit", nil)
			return
		}
		if limit > 200 {
			limit = 200
		}
		before := time.Now().UTC().Add(24 * time.Hour)
		if b := q.Get("before"); b != "" {
			t, err := time.Parse(time.RFC3339, b)
			if err != nil {
				writeError(w, http.StatusBadRequest, CodeValidationFailed, "invalid before", nil)
				return
			}
			before = t
		}
		rows, err := h.q.ListResultsBefore(r.Context(), store.ListResultsBeforeParams{
			CheckID:   id,
			CheckedAt: before,
			Limit:     limit,
		})
		if err != nil {
			h.logger.Error("results before failed", "id", id, "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		out := make([]checkResultRow, 0, len(rows))
		for _, rr := range rows {
			row := checkResultRow{CheckedAt: rr.CheckedAt, Status: rr.Status}
			if rr.LatencyMs.Valid {
				v := rr.LatencyMs.Int64
				row.LatencyMs = &v
			}
			if rr.ErrorMessage.Valid {
				v := rr.ErrorMessage.String
				row.ErrorMessage = &v
			}
			out = append(out, row)
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": out})
		return
	}

	hours := int64(24)
	if h := q.Get("hours"); h != "" {
		if v, err := strconv.ParseInt(h, 10, 64); err == nil && v > 0 {
			hours = v
		}
	}
	if hours > 720 {
		hours = 720
	}
	end := time.Now().UTC()
	start := end.Add(-time.Duration(hours) * time.Hour)

	rows, err := h.q.GetResultsInRange(r.Context(), store.GetResultsInRangeParams{
		CheckID:     id,
		CheckedAt:   start,
		CheckedAt_2: end,
	})
	if err != nil {
		h.logger.Error("results range failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if len(rows) > 2000 {
		rows = rows[len(rows)-2000:]
	}
	out := make([]checkResultRow, 0, len(rows))
	for _, rr := range rows {
		row := checkResultRow{
			CheckedAt: rr.CheckedAt,
			Status:    rr.Status,
		}
		if rr.LatencyMs.Valid {
			v := rr.LatencyMs.Int64
			row.LatencyMs = &v
		}
		if rr.ErrorMessage.Valid {
			v := rr.ErrorMessage.String
			row.ErrorMessage = &v
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

func parseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid id", nil)
		return 0, false
	}
	return id, true
}

func validateCheckParams(name, typ string, interval, timeout, retries, fail, recover int64) map[string]string {
	f := map[string]string{}
	if name == "" {
		f["name"] = FieldRequired
	} else if len(name) > 200 {
		f["name"] = FieldTooLong
	}
	if _, ok := validCheckTypes[typ]; !ok {
		f["type"] = FieldInvalidValue
	}
	if interval < 10 || interval > 86400 {
		f["interval_seconds"] = FieldOutOfRange
	}
	if timeout < 1 || timeout > 300 {
		f["timeout_seconds"] = FieldOutOfRange
	}
	if timeout > interval {
		f["timeout_seconds"] = FieldOutOfRange
	}
	if retries < 0 || retries > 10 {
		f["retries"] = FieldOutOfRange
	}
	if fail < 1 || fail > 20 {
		f["failure_threshold"] = FieldOutOfRange
	}
	if recover < 1 || recover > 20 {
		f["recovery_threshold"] = FieldOutOfRange
	}
	return f
}

func validReopenMode(m string) bool {
	switch m {
	case "always", "never", "flapping_only":
		return true
	}
	return false
}

func (h *ChecksHandler) runOnceAfterCreate(c store.Check) {
	if h.registry == nil || h.incidentSvc == nil {
		return
	}
	if c.Type == "push" || !c.Enabled {
		return
	}
	checker, ok := h.registry.Get(check.Type(c.Type))
	if !ok {
		return
	}
	go func(c store.Check) {
		timeout := time.Duration(c.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		runCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result := checker.Run(runCtx, []byte(c.ConfigJson))
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer persistCancel()
		if err := check.PersistResult(persistCtx, h.db, h.q, h.incidentSvc, c, result); err != nil {
			h.logger.Warn("run-on-create persist failed", "id", c.ID, "err", err)
		}
	}(c)
}
