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

	"github.com/TRC-Loop/cairn/internal/statuspage"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
)

type StatusPageDomainsHandler struct {
	q      *store.Queries
	svc    *statuspage.Service
	cache  *statuspage.DomainCache
	logger *slog.Logger
}

func NewStatusPageDomainsHandler(q *store.Queries, svc *statuspage.Service, cache *statuspage.DomainCache, logger *slog.Logger) *StatusPageDomainsHandler {
	return &StatusPageDomainsHandler{q: q, svc: svc, cache: cache, logger: logger}
}

type domainResponse struct {
	ID        int64     `json:"id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

func toDomainResponse(d store.StatusPageDomain) domainResponse {
	return domainResponse{ID: d.ID, Domain: d.Domain, CreatedAt: d.CreatedAt}
}

func (h *StatusPageDomainsHandler) List(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if _, err := h.q.GetStatusPage(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	rows, err := h.svc.ListDomains(r.Context(), id)
	if err != nil {
		h.logger.Error("list domains failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]domainResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toDomainResponse(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": out})
}

type addDomainRequest struct {
	Domain string `json:"domain"`
}

func (h *StatusPageDomainsHandler) Add(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if _, err := h.q.GetStatusPage(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	var req addDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	row, err := h.svc.AddDomain(r.Context(), id, req.Domain)
	if err != nil {
		switch {
		case errors.Is(err, statuspage.ErrDomainInvalidFormat):
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"domain": FieldInvalidFormat})
			return
		case errors.Is(err, statuspage.ErrDomainConflict):
			writeError(w, http.StatusConflict, CodeConflict, "domain already assigned", map[string]string{"domain": "conflict"})
			return
		}
		h.logger.Error("add domain failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.cache.Reload(r.Context(), h.q); err != nil {
		h.logger.Warn("domain cache reload failed", "err", err)
	}
	h.logger.Info("status page domain added", "status_page_id", id, "domain", row.Domain)
	writeJSON(w, http.StatusCreated, map[string]any{"domain": toDomainResponse(row)})
}

func (h *StatusPageDomainsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	dIDStr := chi.URLParam(r, "domain_id")
	dID, err := strconv.ParseInt(dIDStr, 10, 64)
	if err != nil || dID <= 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "invalid domain_id", nil)
		return
	}
	if err := h.svc.RemoveDomain(r.Context(), id, dID); err != nil {
		if errors.Is(err, statuspage.ErrDomainNotFound) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("remove domain failed", "id", id, "domain_id", dID, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if err := h.cache.Reload(r.Context(), h.q); err != nil {
		h.logger.Warn("domain cache reload failed", "err", err)
	}
	h.logger.Info("status page domain removed", "status_page_id", id, "domain_id", dID)
	w.WriteHeader(http.StatusNoContent)
}
