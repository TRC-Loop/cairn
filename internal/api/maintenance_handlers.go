// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/maintenance"
	"github.com/TRC-Loop/cairn/internal/store"
)

type MaintenanceHandler struct {
	q      *store.Queries
	svc    *maintenance.Service
	logger *slog.Logger
}

func NewMaintenanceHandler(q *store.Queries, svc *maintenance.Service, logger *slog.Logger) *MaintenanceHandler {
	return &MaintenanceHandler{q: q, svc: svc, logger: logger}
}

type maintenanceResponse struct {
	ID                      int64     `json:"id"`
	Title                   string    `json:"title"`
	Description             string    `json:"description"`
	DescriptionHTML         string    `json:"description_html"`
	StartsAt                time.Time `json:"starts_at"`
	EndsAt                  time.Time `json:"ends_at"`
	State                   string    `json:"state"`
	CreatedByUserID         *int64    `json:"created_by_user_id"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
	AffectedComponentIDs    []int64   `json:"affected_component_ids"`
	AffectedComponentNames  []string  `json:"affected_component_names"`
}

type maintenanceComponentResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	DisplayOrder int64  `json:"display_order"`
}

func toMaintenanceResponse(w store.MaintenanceWindow, components []store.Component) maintenanceResponse {
	desc := ""
	if w.Description.Valid {
		desc = w.Description.String
	}
	ids := make([]int64, 0, len(components))
	names := make([]string, 0, len(components))
	for _, c := range components {
		ids = append(ids, c.ID)
		names = append(names, c.Name)
	}
	out := maintenanceResponse{
		ID:                     w.ID,
		Title:                  w.Title,
		Description:            desc,
		DescriptionHTML:        string(incident.RenderMarkdown(desc)),
		StartsAt:               w.StartsAt,
		EndsAt:                 w.EndsAt,
		State:                  w.State,
		CreatedAt:              w.CreatedAt,
		UpdatedAt:              w.UpdatedAt,
		AffectedComponentIDs:   ids,
		AffectedComponentNames: names,
	}
	if w.CreatedByUserID.Valid {
		v := w.CreatedByUserID.Int64
		out.CreatedByUserID = &v
	}
	return out
}

func toMaintenanceComponentResponse(c store.Component) maintenanceComponentResponse {
	desc := ""
	if c.Description.Valid {
		desc = c.Description.String
	}
	return maintenanceComponentResponse{
		ID:           c.ID,
		Name:         c.Name,
		Description:  desc,
		DisplayOrder: c.DisplayOrder,
	}
}

func (h *MaintenanceHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := maintenance.ListFilter{
		Status: q.Get("status"),
	}
	if q.Get("upcoming") == "1" {
		filter.Upcoming = true
	}
	if v := q.Get("past_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.PastDays = n
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.Offset = n
		}
	}
	switch filter.Status {
	case "", "all", "scheduled", "in_progress", "completed", "cancelled":
	default:
		writeError(w, http.StatusBadRequest, CodeBadRequest, "invalid status", nil)
		return
	}

	rows, total, err := h.svc.ListFiltered(r.Context(), filter)
	if err != nil {
		h.logger.Error("list maintenance failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]maintenanceResponse, 0, len(rows))
	for _, mw := range rows {
		comps, _ := h.svc.AffectedComponents(r.Context(), mw.ID)
		out = append(out, toMaintenanceResponse(mw, comps))
	}
	writeJSON(w, http.StatusOK, map[string]any{"maintenance": out, "total": total})
}

func (h *MaintenanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	mw, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get maintenance failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	comps, err := h.svc.AffectedComponents(r.Context(), id)
	if err != nil {
		h.logger.Error("list affected components failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	compsOut := make([]maintenanceComponentResponse, 0, len(comps))
	for _, c := range comps {
		compsOut = append(compsOut, toMaintenanceComponentResponse(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"window":              toMaintenanceResponse(mw, comps),
		"affected_components": compsOut,
	})
}

type maintenanceCreateRequest struct {
	Title                string  `json:"title"`
	Description          string  `json:"description"`
	StartsAt             string  `json:"starts_at"`
	EndsAt               string  `json:"ends_at"`
	AffectedComponentIDs []int64 `json:"affected_component_ids"`
}

func (h *MaintenanceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req maintenanceCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	fields := map[string]string{}
	if l := len(req.Title); l < 1 {
		fields["title"] = FieldRequired
	} else if l > 200 {
		fields["title"] = FieldTooLong
	}
	if len(req.Description) > 2000 {
		fields["description"] = FieldTooLong
	}
	startsAt, errStart := time.Parse(time.RFC3339, req.StartsAt)
	if req.StartsAt == "" {
		fields["starts_at"] = FieldRequired
	} else if errStart != nil {
		fields["starts_at"] = FieldInvalidFormat
	}
	endsAt, errEnd := time.Parse(time.RFC3339, req.EndsAt)
	if req.EndsAt == "" {
		fields["ends_at"] = FieldRequired
	} else if errEnd != nil {
		fields["ends_at"] = FieldInvalidFormat
	}
	if errStart == nil && errEnd == nil && !endsAt.After(startsAt) {
		fields["ends_at"] = FieldInvalidValue
	}
	if errEnd == nil && !endsAt.After(time.Now().UTC()) {
		fields["ends_at"] = FieldOutOfRange
	}
	if len(req.AffectedComponentIDs) < 1 {
		fields["affected_component_ids"] = FieldRequired
	} else {
		for _, id := range req.AffectedComponentIDs {
			if _, err := h.q.GetComponent(r.Context(), id); err != nil {
				fields["affected_component_ids"] = FieldNotFound
				break
			}
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	user, _ := auth.UserFromContext(r.Context())
	mw, err := h.svc.Create(r.Context(), maintenance.CreateInput{
		Title:              req.Title,
		Description:        req.Description,
		StartsAt:           startsAt,
		EndsAt:             endsAt,
		CreatedByUserID:    user.ID,
		AffectedComponents: req.AffectedComponentIDs,
	})
	if err != nil {
		h.logger.Error("create maintenance failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	comps, _ := h.svc.AffectedComponents(r.Context(), mw.ID)
	h.logger.Info("maintenance created", "id", mw.ID, "user_id", user.ID, "state", mw.State)
	writeJSON(w, http.StatusCreated, map[string]any{
		"window": toMaintenanceResponse(mw, comps),
	})
}

type maintenancePatchRequest struct {
	Title                *string  `json:"title"`
	Description          *string  `json:"description"`
	StartsAt             *string  `json:"starts_at"`
	EndsAt               *string  `json:"ends_at"`
	AffectedComponentIDs *[]int64 `json:"affected_component_ids"`
}

func (h *MaintenanceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req maintenancePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	in := maintenance.UpdateInput{
		Title:              req.Title,
		Description:        req.Description,
		AffectedComponents: req.AffectedComponentIDs,
	}
	fields := map[string]string{}
	if req.Title != nil {
		if l := len(*req.Title); l < 1 {
			fields["title"] = FieldRequired
		} else if l > 200 {
			fields["title"] = FieldTooLong
		}
	}
	if req.Description != nil && len(*req.Description) > 2000 {
		fields["description"] = FieldTooLong
	}
	if req.StartsAt != nil {
		t, err := time.Parse(time.RFC3339, *req.StartsAt)
		if err != nil {
			fields["starts_at"] = FieldInvalidFormat
		} else {
			in.StartsAt = &t
		}
	}
	if req.EndsAt != nil {
		t, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err != nil {
			fields["ends_at"] = FieldInvalidFormat
		} else {
			in.EndsAt = &t
		}
	}
	if req.AffectedComponentIDs != nil {
		if len(*req.AffectedComponentIDs) < 1 {
			fields["affected_component_ids"] = FieldRequired
		} else {
			for _, cid := range *req.AffectedComponentIDs {
				if _, err := h.q.GetComponent(r.Context(), cid); err != nil {
					fields["affected_component_ids"] = FieldNotFound
					break
				}
			}
		}
	}
	if len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}

	updated, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		if errors.Is(err, maintenance.ErrAlreadyEnded) {
			writeError(w, http.StatusBadRequest, CodeBadRequest, "this maintenance window has ended and can't be edited", nil)
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	comps, _ := h.svc.AffectedComponents(r.Context(), id)
	h.logger.Info("maintenance updated", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{
		"window": toMaintenanceResponse(updated, comps),
	})
}

func (h *MaintenanceHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.Cancel(r.Context(), id); err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	mw, _ := h.svc.Get(r.Context(), id)
	comps, _ := h.svc.AffectedComponents(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{"window": toMaintenanceResponse(mw, comps)})
}

func (h *MaintenanceHandler) EndNow(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.EndNow(r.Context(), id); err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	mw, _ := h.svc.Get(r.Context(), id)
	comps, _ := h.svc.AffectedComponents(r.Context(), id)
	writeJSON(w, http.StatusOK, map[string]any{"window": toMaintenanceResponse(mw, comps)})
}

func (h *MaintenanceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, maintenance.ErrNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	h.logger.Info("maintenance deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}
