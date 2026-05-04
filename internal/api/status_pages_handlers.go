// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/TRC-Loop/cairn/internal/statuspage"
	"github.com/TRC-Loop/cairn/internal/store"
)

type StatusPagesHandler struct {
	q      *store.Queries
	svc    *statuspage.Service
	logger *slog.Logger
}

func NewStatusPagesHandler(q *store.Queries, svc *statuspage.Service, logger *slog.Logger) *StatusPagesHandler {
	return &StatusPagesHandler{q: q, svc: svc, logger: logger}
}

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

type statusPageResponse struct {
	ID               int64     `json:"id"`
	Slug             string    `json:"slug"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	LogoURL          string    `json:"logo_url"`
	AccentColor      string    `json:"accent_color"`
	CustomFooterHTML string    `json:"custom_footer_html"`
	FooterMode       string    `json:"footer_mode"`
	PasswordSet      bool      `json:"password_set"`
	IsDefault        bool      `json:"is_default"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toStatusPageResponse(p store.StatusPage) statusPageResponse {
	return statusPageResponse{
		ID:               p.ID,
		Slug:             p.Slug,
		Title:            p.Title,
		Description:      nullStringValue(p.Description),
		LogoURL:          nullStringValue(p.LogoUrl),
		AccentColor:      nullStringValue(p.AccentColor),
		CustomFooterHTML: nullStringValue(p.CustomFooterHtml),
		FooterMode:       p.FooterMode,
		PasswordSet:      p.PasswordHash.Valid && p.PasswordHash.String != "",
		IsDefault:        p.IsDefault,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}
}

func nullStringValue(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

type statusPageWriteRequest struct {
	Slug             *string `json:"slug"`
	Title            *string `json:"title"`
	Description      *string `json:"description"`
	LogoURL          *string `json:"logo_url"`
	AccentColor      *string `json:"accent_color"`
	CustomFooterHTML *string `json:"custom_footer_html"`
	IsDefault        *bool   `json:"is_default"`
}

func (h *StatusPagesHandler) List(w http.ResponseWriter, r *http.Request) {
	pages, err := h.q.ListStatusPages(r.Context())
	if err != nil {
		h.logger.Error("list status pages failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]statusPageResponse, 0, len(pages))
	for _, p := range pages {
		out = append(out, toStatusPageResponse(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"status_pages": out, "total": len(out)})
}

func (h *StatusPagesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	p, err := h.q.GetStatusPage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get status page failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	comps, err := h.q.ListComponentsForStatusPage(r.Context(), id)
	if err != nil {
		h.logger.Error("list status page components failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	compsOut := make([]componentResponse, 0, len(comps))
	for _, c := range comps {
		count, _ := h.q.CountChecksForComponent(r.Context(), sql.NullInt64{Int64: c.ID, Valid: true})
		compsOut = append(compsOut, toComponentResponse(c, count))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status_page": toStatusPageResponse(p),
		"components":  compsOut,
	})
}

func validateStatusPageWrite(slug, title string) map[string]string {
	f := map[string]string{}
	if slug != "" && !slugRe.MatchString(slug) {
		f["slug"] = FieldInvalidFormat
	}
	if title == "" {
		f["title"] = FieldRequired
	} else if len(title) > 200 {
		f["title"] = FieldTooLong
	}
	return f
}

func (h *StatusPagesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req statusPageWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	slug := ""
	if req.Slug != nil {
		slug = *req.Slug
	}
	title := ""
	if req.Title != nil {
		title = *req.Title
	}
	if fields := validateStatusPageWrite(slug, title); len(fields) > 0 || slug == "" {
		if slug == "" {
			fields["slug"] = FieldRequired
		}
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	if _, err := h.q.GetStatusPageBySlug(r.Context(), slug); err == nil {
		writeError(w, http.StatusConflict, CodeConflict, "slug already in use", map[string]string{"slug": "conflict"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		h.logger.Error("slug check failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	in := statuspage.CreateInput{Slug: slug, Title: title}
	if req.Description != nil {
		in.Description = *req.Description
	}
	if req.LogoURL != nil {
		in.LogoURL = *req.LogoURL
	}
	if req.AccentColor != nil {
		in.AccentColor = *req.AccentColor
	}
	if req.CustomFooterHTML != nil {
		in.CustomFooterHTML = *req.CustomFooterHTML
	}
	if req.IsDefault != nil {
		in.IsDefault = *req.IsDefault
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.logger.Error("create status page failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("status page created", "id", p.ID, "slug", p.Slug)
	writeJSON(w, http.StatusCreated, map[string]any{"status_page": toStatusPageResponse(p)})
}

func (h *StatusPagesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	existing, err := h.q.GetStatusPage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	var req statusPageWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if req.Slug != nil && *req.Slug != existing.Slug {
		h.logger.Warn("status page slug change attempted (unsupported)", "id", id, "from", existing.Slug, "to", *req.Slug)
	}
	in := statuspage.UpdateInput{
		Title:            existing.Title,
		Description:      nullStringValue(existing.Description),
		LogoURL:          nullStringValue(existing.LogoUrl),
		AccentColor:      nullStringValue(existing.AccentColor),
		CustomFooterHTML: nullStringValue(existing.CustomFooterHtml),
	}
	if req.Title != nil {
		in.Title = *req.Title
	}
	if req.Description != nil {
		in.Description = *req.Description
	}
	if req.LogoURL != nil {
		in.LogoURL = *req.LogoURL
	}
	if req.AccentColor != nil {
		in.AccentColor = *req.AccentColor
	}
	if req.CustomFooterHTML != nil {
		in.CustomFooterHTML = *req.CustomFooterHTML
	}
	if fields := validateStatusPageWrite("", in.Title); len(fields) > 0 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
		return
	}
	if err := h.svc.Update(r.Context(), id, in); err != nil {
		h.logger.Error("update status page failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	updated, _ := h.q.GetStatusPage(r.Context(), id)
	h.logger.Info("status page updated", "id", id)
	writeJSON(w, http.StatusOK, map[string]any{"status_page": toStatusPageResponse(updated)})
}

func (h *StatusPagesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.logger.Error("delete status page failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("status page deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *StatusPagesHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.SetDefault(r.Context(), id); err != nil {
		h.logger.Error("set default status page failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("status page set as default", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

type setPasswordRequest struct {
	Password *string `json:"password"`
}

func (h *StatusPagesHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
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
	var req setPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	plaintext := ""
	if req.Password != nil {
		plaintext = *req.Password
	}
	if plaintext != "" && len(plaintext) < 8 {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"password": FieldTooShort})
		return
	}
	if err := h.svc.SetPassword(r.Context(), id, plaintext); err != nil {
		h.logger.Error("set status page password failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.logger.Info("status page password updated", "id", id, "cleared", plaintext == "")
	w.WriteHeader(http.StatusNoContent)
}

type footerElementResponse struct {
	ID           int64  `json:"id"`
	ElementType  string `json:"element_type"`
	Label        string `json:"label,omitempty"`
	URL          string `json:"url,omitempty"`
	OpenInNewTab bool   `json:"open_in_new_tab"`
	DisplayOrder int64  `json:"display_order"`
}

func toFooterElementResponse(e store.StatusPageFooterElement) footerElementResponse {
	return footerElementResponse{
		ID:           e.ID,
		ElementType:  e.ElementType,
		Label:        nullStringValue(e.Label),
		URL:          nullStringValue(e.Url),
		OpenInNewTab: e.OpenInNewTab,
		DisplayOrder: e.DisplayOrder,
	}
}

type footerElementInputRequest struct {
	ElementType  string `json:"element_type"`
	Label        string `json:"label"`
	URL          string `json:"url"`
	OpenInNewTab *bool  `json:"open_in_new_tab"`
}

func (h *StatusPagesHandler) GetFooter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	p, err := h.q.GetStatusPage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		h.logger.Error("get status page footer failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	elements, err := h.svc.ListFooterElements(r.Context(), id)
	if err != nil {
		h.logger.Error("list footer elements failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	out := make([]footerElementResponse, 0, len(elements))
	for _, el := range elements {
		out = append(out, toFooterElementResponse(el))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"footer_mode":        p.FooterMode,
		"elements":           out,
		"custom_footer_html": nullStringValue(p.CustomFooterHtml),
	})
}

type replaceFooterElementsRequest struct {
	Elements []footerElementInputRequest `json:"elements"`
}

func (h *StatusPagesHandler) ReplaceFooterElements(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	page, err := h.q.GetStatusPage(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, CodeNotFound, "not found", nil)
			return
		}
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	var req replaceFooterElementsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	inputs := make([]statuspage.FooterElementInput, 0, len(req.Elements))
	for _, el := range req.Elements {
		newTab := true
		if el.OpenInNewTab != nil {
			newTab = *el.OpenInNewTab
		}
		inputs = append(inputs, statuspage.FooterElementInput{
			ElementType:  el.ElementType,
			Label:        el.Label,
			URL:          el.URL,
			OpenInNewTab: newTab,
		})
	}
	// Auto-switch to 'structured' when adding first elements to a page stuck in 'html'
	// mode with no custom HTML (e.g. after clearing legacy HTML content).
	autoSwitch := page.FooterMode == statuspage.FooterModeHTML &&
		!page.CustomFooterHtml.Valid &&
		len(req.Elements) > 0
	if autoSwitch {
		prior, err := h.svc.ListFooterElements(r.Context(), id)
		if err != nil || len(prior) > 0 {
			autoSwitch = false
		}
	}
	saved, err := h.svc.ReplaceFooterElements(r.Context(), id, inputs)
	if err != nil {
		var fe *statuspage.ErrFooterElement
		if errors.As(err, &fe) {
			fields := map[string]string{
				fmt.Sprintf("elements[%d].%s", fe.Index, fe.Field): fe.Code,
			}
			writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", fields)
			return
		}
		h.logger.Error("replace footer elements failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	if autoSwitch {
		if err := h.svc.SetFooterMode(r.Context(), id, statuspage.FooterModeStructured); err != nil {
			h.logger.Warn("auto-switch footer mode failed", "id", id, "err", err)
		}
	}
	out := make([]footerElementResponse, 0, len(saved))
	for _, el := range saved {
		out = append(out, toFooterElementResponse(el))
	}
	h.logger.Info("status page footer elements updated", "id", id, "count", len(saved))
	writeJSON(w, http.StatusOK, map[string]any{"elements": out})
}

type setFooterModeRequest struct {
	FooterMode string `json:"footer_mode"`
}

func (h *StatusPagesHandler) SetFooterMode(w http.ResponseWriter, r *http.Request) {
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
	var req setFooterModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if err := h.svc.SetFooterMode(r.Context(), id, req.FooterMode); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "validation failed", map[string]string{"footer_mode": FieldInvalidValue})
		return
	}
	h.logger.Info("status page footer mode updated", "id", id, "mode", req.FooterMode)
	w.WriteHeader(http.StatusNoContent)
}

type setComponentsRequest struct {
	ComponentIDs []int64 `json:"component_ids"`
}

func (h *StatusPagesHandler) SetComponents(w http.ResponseWriter, r *http.Request) {
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
	var req setComponentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidJSON, "invalid json", nil)
		return
	}
	if err := h.q.RemoveAllComponentsFromStatusPage(r.Context(), id); err != nil {
		h.logger.Error("clear status page components failed", "id", id, "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	for idx, cid := range req.ComponentIDs {
		if err := h.svc.AddComponent(r.Context(), id, cid, int64(idx)); err != nil {
			h.logger.Error("add component to status page failed", "id", id, "component_id", cid, "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
	}
	h.logger.Info("status page components updated", "id", id, "count", len(req.ComponentIDs))
	w.WriteHeader(http.StatusNoContent)
}
