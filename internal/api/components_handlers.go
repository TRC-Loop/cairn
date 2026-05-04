// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/TRC-Loop/cairn/internal/component"
	"github.com/TRC-Loop/cairn/internal/store"
)

type ComponentsHandler struct {
	q      *store.Queries
	svc    *component.Service
	logger *slog.Logger
}

func NewComponentsHandler(q *store.Queries, svc *component.Service, logger *slog.Logger) *ComponentsHandler {
	return &ComponentsHandler{q: q, svc: svc, logger: logger}
}

type componentResponse struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	DisplayOrder int64     `json:"display_order"`
	CheckCount   int64     `json:"check_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func toComponentResponse(c store.Component, count int64) componentResponse {
	desc := ""
	if c.Description.Valid {
		desc = c.Description.String
	}
	return componentResponse{
		ID:           c.ID,
		Name:         c.Name,
		Description:  desc,
		DisplayOrder: c.DisplayOrder,
		CheckCount:   count,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

type componentWriteRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	DisplayOrder *int64  `json:"display_order"`
}

func (h *ComponentsHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListComponents(r.Context())
	if err != nil {
		h.logger.Error("list components failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]componentResponse, 0, len(rows))
	for _, c := range rows {
		count, _ := h.q.CountChecksForComponent(r.Context(), sql.NullInt64{Int64: c.ID, Valid: true})
		out = append(out, toComponentResponse(c, count))
	}
	writeJSON(w, http.StatusOK, map[string]any{"components": out, "total": len(out)})
}

func (h *ComponentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	c, err := h.q.GetComponent(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get component failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	checks, err := h.q.ListChecksForComponent(r.Context(), sql.NullInt64{Int64: id, Valid: true})
	if err != nil {
		h.logger.Error("list component checks failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	checksOut := make([]checkResponse, 0, len(checks))
	for _, ch := range checks {
		checksOut = append(checksOut, toCheckResponse(ch, false))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"component": toComponentResponse(c, int64(len(checks))),
		"checks":    checksOut,
	})
}

func validateComponentParams(name, description string, displayOrder int64) map[string]string {
	f := map[string]string{}
	if name == "" {
		f["name"] = FieldRequired
	} else if len(name) > 100 {
		f["name"] = FieldTooLong
	}
	if len(description) > 500 {
		f["description"] = FieldTooLong
	}
	if displayOrder < 0 || displayOrder > 999 {
		f["display_order"] = FieldOutOfRange
	}
	return f
}

func (h *ComponentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req componentWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if req.Name == nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"name": FieldRequired})
		return
	}
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	var order int64
	if req.DisplayOrder != nil {
		order = *req.DisplayOrder
	}
	if fields := validateComponentParams(*req.Name, desc, order); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	c, err := h.svc.Create(r.Context(), component.CreateInput{
		Name:         *req.Name,
		Description:  desc,
		DisplayOrder: order,
	})
	if err != nil {
		h.logger.Error("create component failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("component created", "id", c.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"component": toComponentResponse(c, 0)})
}

func (h *ComponentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	existing, err := h.q.GetComponent(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get component failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}

	var req componentWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}

	name := existing.Name
	desc := ""
	if existing.Description.Valid {
		desc = existing.Description.String
	}
	order := existing.DisplayOrder
	if req.Name != nil {
		name = *req.Name
	}
	if req.Description != nil {
		desc = *req.Description
	}
	if req.DisplayOrder != nil {
		order = *req.DisplayOrder
	}
	if fields := validateComponentParams(name, desc, order); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	if err := h.svc.Update(r.Context(), id, component.UpdateInput{
		Name:         name,
		Description:  desc,
		DisplayOrder: order,
	}); err != nil {
		h.logger.Error("update component failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	updated, _ := h.q.GetComponent(r.Context(), id)
	count, _ := h.q.CountChecksForComponent(r.Context(), sql.NullInt64{Int64: id, Valid: true})
	h.logger.Info("component updated", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"component": toComponentResponse(updated, count)})
}

func (h *ComponentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.q.DeleteComponent(r.Context(), id); err != nil {
		h.logger.Error("delete component failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("component deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

type componentReorderRequest struct {
	DisplayOrder *int64 `json:"display_order"`
}

func (h *ComponentsHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req componentReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DisplayOrder == nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	existing, err := h.q.GetComponent(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if *req.DisplayOrder < 0 || *req.DisplayOrder > 999 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"display_order": FieldOutOfRange})
		return
	}
	desc := ""
	if existing.Description.Valid {
		desc = existing.Description.String
	}
	if err := h.svc.Update(r.Context(), id, component.UpdateInput{
		Name:         existing.Name,
		Description:  desc,
		DisplayOrder: *req.DisplayOrder,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
