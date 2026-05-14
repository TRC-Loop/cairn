// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/component"
	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/maintenance"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
)

const (
	historyDays        = 90
	unlockCookiePrefix = "cairn_page_unlock_"
	unlockCookieMaxAge = 24 * 60 * 60
	unlockKeyLabel     = "cairn-unlock-v1"
	maxUnlockFails     = 5
	unlockLockoutFor   = time.Minute
	unlockWindow       = 10 * time.Minute
)

type Handler struct {
	service     *Service
	component   *component.Service
	maintenance *maintenance.Service
	incident    *incident.Service
	q           *store.Queries
	logger      *slog.Logger
	templates   *template.Template

	signingKey  []byte
	limiter     *unlockLimiter
	domainCache *DomainCache
	trustProxy  bool
}

// NewHandler builds the public status-page HTTP handler. encryptionKey is
// the CAIRN_ENCRYPTION_KEY operator-provided secret; it's salted to derive
// a per-purpose signing key for unlock cookies so a leak of one cookie
// cannot be replayed as a session cookie or vice versa.
func NewHandler(
	service *Service,
	componentSvc *component.Service,
	maintenanceSvc *maintenance.Service,
	incidentSvc *incident.Service,
	q *store.Queries,
	logger *slog.Logger,
	encryptionKey string,
	domainCache *DomainCache,
	trustProxy bool,
) *Handler {
	h := sha256.Sum256([]byte(encryptionKey + "|" + unlockKeyLabel))
	if domainCache == nil {
		domainCache = NewDomainCache()
	}
	return &Handler{
		service:     service,
		component:   componentSvc,
		maintenance: maintenanceSvc,
		incident:    incidentSvc,
		q:           q,
		logger:      logger,
		templates:   mustLoadTemplates(),
		signingKey:  h[:],
		limiter:     newUnlockLimiter(),
		domainCache: domainCache,
		trustProxy:  trustProxy,
	}
}

func (h *Handler) requestHost(r *http.Request) string {
	host := ""
	if h.trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
			if i := strings.Index(forwarded, ","); i != -1 {
				forwarded = forwarded[:i]
			}
			host = strings.TrimSpace(forwarded)
		}
	}
	if host == "" {
		host = r.Host
	}
	if i := strings.LastIndex(host, ":"); i != -1 {
		if _, err := strconv.Atoi(host[i+1:]); err == nil {
			host = host[:i]
		}
	}
	return strings.ToLower(host)
}

// --- Page-view model (shared by HTML and JSON renderers) ---

type pageView struct {
	PageTitle        string
	PageDescription  string
	Slug             string
	LogoURL          string
	CustomFooterHTML string
	FooterHTML       template.HTML
	UpdatedAt        time.Time
	Lang             string
	OverallStatus    string
	OverallStatusFavicon string
	Components       []componentView
	ActiveIncidents  []incidentView
	ActiveMaintenance []maintenanceView
	RecentIncidents  []store.Incident
	// only set on the unlock page
	UnlockError    string
	HidePoweredBy  bool
	ShowHistory    bool
	DirectMonitors []directMonitorView
}

type directMonitorView struct {
	ID            int64       `json:"id"`
	Name          string      `json:"name"`
	Status        string      `json:"status"`
	History       []DayStatus `json:"history_90d,omitempty"`
	UptimePercent string      `json:"-"`
}

type componentView struct {
	ID               int64               `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description,omitempty"`
	Status           string              `json:"status"`
	UnderMaintenance bool                `json:"under_maintenance"`
	History          []DayStatus         `json:"history_90d"`
	UptimePercent    string              `json:"-"`
	ShowMonitors     string              `json:"show_monitors,omitempty"`
	Monitors         []directMonitorView `json:"monitors,omitempty"`
}

type incidentView struct {
	ID        int64              `json:"id"`
	Title     string             `json:"title"`
	Severity  string             `json:"severity"`
	Status    string             `json:"status"`
	StartedAt time.Time          `json:"started_at"`
	Updates   []incidentUpdateVw `json:"updates,omitempty"`
}

type incidentUpdateVw struct {
	Status      string        `json:"status"`
	Message     string        `json:"message"`
	MessageHTML template.HTML `json:"-"`
	CreatedAt   time.Time     `json:"created_at"`
}

type maintenanceView struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	State       string    `json:"state"`
	InProgress  bool      `json:"in_progress"`
}

// --- Handlers ---

func (h *Handler) ServeDefault(w http.ResponseWriter, r *http.Request) {
	host := h.requestHost(r)
	if host != "" {
		if pageID, ok := h.domainCache.Get(host); ok {
			page, err := h.service.Get(r.Context(), pageID)
			if err == nil {
				h.servePage(w, r, page)
				return
			}
			h.logger.Warn("domain-resolved page lookup failed", "domain", host, "page_id", pageID, "err", err)
		}
	}
	page, err := h.service.GetDefault(r.Context())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.renderNotFound(w, r)
			return
		}
		h.logger.Error("get default status page failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.servePage(w, r, page)
}

func (h *Handler) ServeBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		h.renderNotFound(w, r)
		return
	}
	page, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.renderNotFound(w, r)
			return
		}
		h.logger.Error("get status page failed", "slug", slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.servePage(w, r, page)
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request, page store.StatusPage) {
	if page.PasswordHash.Valid && page.PasswordHash.String != "" {
		if !h.validateUnlockCookie(r, page) {
			h.renderUnlock(w, r, page, "")
			return
		}
	}

	view, err := h.buildPageView(r.Context(), page, r.Header.Get("Accept-Language"))
	if err != nil {
		h.logger.Error("build page view failed", "slug", page.Slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.writeSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := h.templates.ExecuteTemplate(w, "status.html", view); err != nil {
		h.logger.Error("render status page failed", "slug", page.Slug, "err", err)
	}
}

func (h *Handler) ServeJSON(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
			return
		}
		h.logger.Error("get status page for json failed", "slug", slug, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	if page.PasswordHash.Valid && page.PasswordHash.String != "" {
		if !h.validateUnlockCookie(r, page) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "locked"})
			return
		}
	}
	view, err := h.buildPageView(r.Context(), page, r.Header.Get("Accept-Language"))
	if err != nil {
		h.logger.Error("build json view failed", "slug", slug, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}

	out := map[string]any{
		"page": map[string]any{
			"title":       view.PageTitle,
			"description": view.PageDescription,
			"slug":        view.Slug,
			"updated_at":  view.UpdatedAt.UTC().Format(time.RFC3339),
		},
		"overall_status":             view.OverallStatus,
		"components":                 view.Components,
		"active_incidents":           view.ActiveIncidents,
		"active_maintenance":         view.ActiveMaintenance,
		"recent_resolved_incidents":  simplifyIncidents(view.RecentIncidents),
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, out)
}

// HandleUnlock accepts the password POST. Correct → sets cookie, 303 to page.
// Wrong → re-renders unlock with an error. Rate-limited per client IP.
func (h *Handler) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	page, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.renderNotFound(w, r)
			return
		}
		h.logger.Error("unlock get page failed", "slug", slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderUnlock(w, r, page, "Invalid form submission.")
		return
	}
	ip := clientIP(r)
	if !h.limiter.allow(ip) {
		h.renderUnlock(w, r, page, "Too many attempts. Wait a minute and try again.")
		return
	}
	pw := r.FormValue("password")
	if pw == "" {
		h.renderUnlock(w, r, page, "Password required.")
		return
	}
	ok, err := h.service.VerifyPassword(r.Context(), slug, pw)
	if err != nil {
		h.logger.Error("verify password failed", "slug", slug, "err", err)
		h.renderUnlock(w, r, page, "Verification error.")
		return
	}
	if !ok {
		h.limiter.recordFail(ip)
		h.renderUnlock(w, r, page, "Incorrect password.")
		return
	}
	h.limiter.recordSuccess(ip)
	http.SetCookie(w, &http.Cookie{
		Name:     unlockCookieName(page.ID),
		Value:    h.signUnlock(page),
		Path:     "/p/" + page.Slug,
		MaxAge:   unlockCookieMaxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/p/"+page.Slug, http.StatusSeeOther)
}

func (h *Handler) renderUnlock(w http.ResponseWriter, r *http.Request, page store.StatusPage, errMsg string) {
	view := pageView{
		PageTitle:   page.Title,
		Slug:        page.Slug,
		Lang:        preferredLang(r.Header.Get("Accept-Language")),
		UnlockError: errMsg,
		OverallStatus: "unknown",
		OverallStatusFavicon: faviconForStatus("unknown"),
	}
	h.writeSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	if errMsg != "" {
		status = http.StatusUnauthorized
	}
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, "unlock.html", view); err != nil {
		h.logger.Error("render unlock failed", "err", err)
	}
}

func (h *Handler) renderNotFound(w http.ResponseWriter, r *http.Request) {
	view := pageView{
		PageTitle:     "Not found",
		Lang:          preferredLang(r.Header.Get("Accept-Language")),
		OverallStatus: "unknown",
		OverallStatusFavicon: faviconForStatus("unknown"),
	}
	h.writeSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	if err := h.templates.ExecuteTemplate(w, "404.html", view); err != nil {
		h.logger.Error("render 404 failed", "err", err)
	}
}

// --- View builders ---

func (h *Handler) buildPageView(ctx context.Context, page store.StatusPage, acceptLang string) (*pageView, error) {
	components, err := h.service.ListComponents(ctx, page.ID)
	if err != nil {
		return nil, fmt.Errorf("list components: %w", err)
	}

	settings, _ := h.service.ListComponentSettings(ctx, page.ID)
	showMonitorsByID := make(map[int64]string, len(settings))
	for _, s := range settings {
		showMonitorsByID[s.ComponentID] = s.ShowMonitorsDefault
	}

	componentViews := make([]componentView, 0, len(components))
	componentIDSet := make(map[int64]bool, len(components))
	for _, c := range components {
		componentIDSet[c.ID] = true

		status, err := h.component.AggregateStatus(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("aggregate status %d: %w", c.ID, err)
		}
		under, err := h.maintenance.IsComponentUnderMaintenance(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("check maintenance %d: %w", c.ID, err)
		}
		history, err := h.service.HistoryFor(ctx, c.ID, historyDays)
		if err != nil {
			return nil, fmt.Errorf("history %d: %w", c.ID, err)
		}
		displayStatus := string(status)
		if under {
			displayStatus = "maintenance"
		}
		showMode := showMonitorsByID[c.ID]
		if showMode == "" {
			showMode = "off"
		}
		var monitors []directMonitorView
		if showMode != "off" {
			checks, err := h.q.ListChecksForComponent(ctx, sql.NullInt64{Int64: c.ID, Valid: true})
			if err == nil {
				for _, ck := range checks {
					st := ck.LastStatus
					if st == "" {
						st = "unknown"
					}
					mh, _ := h.service.HistoryForCheck(ctx, ck.ID, historyDays)
					monitors = append(monitors, directMonitorView{
						ID:            ck.ID,
						Name:          ck.Name,
						Status:        st,
						History:       mh,
						UptimePercent: UptimePercent(mh),
					})
				}
			}
		}
		componentViews = append(componentViews, componentView{
			ID:               c.ID,
			Name:             c.Name,
			Description:      c.Description.String,
			Status:           displayStatus,
			UnderMaintenance: under,
			History:          history,
			UptimePercent:    UptimePercent(history),
			ShowMonitors:     showMode,
			Monitors:         monitors,
		})
	}

	activeIncidents, err := h.buildActiveIncidents(ctx, componentIDSet)
	if err != nil {
		return nil, err
	}
	activeMaintenance, err := h.buildActiveMaintenance(ctx, componentIDSet)
	if err != nil {
		return nil, err
	}
	recent, err := h.buildRecentResolvedIncidents(ctx, componentIDSet)
	if err != nil {
		return nil, err
	}

	footerElements, err := h.service.ListFooterElements(ctx, page.ID)
	if err != nil {
		return nil, fmt.Errorf("list footer elements: %w", err)
	}
	footerHTML := RenderFooter(page.FooterMode, footerElements, page.CustomFooterHtml.String)

	overall := computeOverall(componentViews, activeIncidents, activeMaintenance)

	return &pageView{
		PageTitle:         page.Title,
		PageDescription:   page.Description.String,
		Slug:              page.Slug,
		LogoURL:           page.LogoUrl.String,
		CustomFooterHTML:  page.CustomFooterHtml.String,
		FooterHTML:        footerHTML,
		UpdatedAt:         time.Now().UTC(),
		Lang:              preferredLang(acceptLang),
		OverallStatus:     overall,
		OverallStatusFavicon: faviconForStatus(overall),
		Components:        componentViews,
		ActiveIncidents:   activeIncidents,
		ActiveMaintenance: activeMaintenance,
		RecentIncidents:   recent,
		HidePoweredBy:     page.HidePoweredBy,
		ShowHistory:       page.ShowHistory,
		DirectMonitors:    h.buildDirectMonitors(ctx, page.ID),
	}, nil
}

func (h *Handler) buildDirectMonitors(ctx context.Context, statusPageID int64) []directMonitorView {
	checks, err := h.service.ListDirectMonitors(ctx, statusPageID)
	if err != nil {
		h.logger.Warn("list direct monitors failed", "page_id", statusPageID, "err", err)
		return nil
	}
	if len(checks) == 0 {
		return nil
	}
	out := make([]directMonitorView, 0, len(checks))
	for _, c := range checks {
		status := c.LastStatus
		if status == "" {
			status = "unknown"
		}
		out = append(out, directMonitorView{ID: c.ID, Name: c.Name, Status: status})
	}
	return out
}

func (h *Handler) buildActiveIncidents(ctx context.Context, pageComponentIDs map[int64]bool) ([]incidentView, error) {
	all, err := h.q.ListActiveIncidents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active incidents: %w", err)
	}
	var views []incidentView
	for _, inc := range all {
		if len(pageComponentIDs) > 0 && !incidentAffectsPage(ctx, h.q, inc.ID, pageComponentIDs) {
			continue
		}
		updates, err := h.q.ListUpdatesForIncident(ctx, inc.ID)
		if err != nil {
			return nil, fmt.Errorf("list updates: %w", err)
		}
		uv := make([]incidentUpdateVw, 0, len(updates))
		for _, u := range updates {
			uv = append(uv, incidentUpdateVw{Status: u.Status, Message: u.Message, MessageHTML: incident.RenderMarkdown(u.Message), CreatedAt: u.CreatedAt})
		}
		views = append(views, incidentView{
			ID:        inc.ID,
			Title:     inc.Title,
			Severity:  inc.Severity,
			Status:    inc.Status,
			StartedAt: inc.StartedAt,
			Updates:   uv,
		})
	}
	return views, nil
}

func (h *Handler) buildActiveMaintenance(ctx context.Context, pageComponentIDs map[int64]bool) ([]maintenanceView, error) {
	active, err := h.maintenance.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active maintenance: %w", err)
	}
	upcoming, err := h.maintenance.ListUpcoming(ctx)
	if err != nil {
		return nil, fmt.Errorf("list upcoming maintenance: %w", err)
	}
	candidates := append([]store.MaintenanceWindow{}, active...)
	candidates = append(candidates, upcoming...)

	var views []maintenanceView
	for _, mw := range candidates {
		ids, err := h.q.ListComponentsForMaintenance(ctx, mw.ID)
		if err != nil {
			return nil, fmt.Errorf("list components for maintenance %d: %w", mw.ID, err)
		}
		if !anyIntersects(ids, pageComponentIDs) {
			continue
		}
		views = append(views, maintenanceView{
			ID:          mw.ID,
			Title:       mw.Title,
			Description: mw.Description.String,
			StartsAt:    mw.StartsAt,
			EndsAt:      mw.EndsAt,
			State:       mw.State,
			InProgress:  mw.State == maintenance.StateInProgress,
		})
		if len(views) >= 3 {
			break
		}
	}
	return views, nil
}

func (h *Handler) buildRecentResolvedIncidents(ctx context.Context, pageComponentIDs map[int64]bool) ([]store.Incident, error) {
	all, err := h.q.ListRecentResolvedIncidents(ctx, 25)
	if err != nil {
		return nil, fmt.Errorf("recent resolved: %w", err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -14)
	var out []store.Incident
	for _, inc := range all {
		if inc.ResolvedAt.Valid && inc.ResolvedAt.Time.Before(cutoff) {
			continue
		}
		if len(pageComponentIDs) > 0 && !incidentAffectsPage(ctx, h.q, inc.ID, pageComponentIDs) {
			continue
		}
		out = append(out, inc)
		if len(out) >= 5 {
			break
		}
	}
	return out, nil
}

func incidentAffectsPage(ctx context.Context, q *store.Queries, incidentID int64, pageComponentIDs map[int64]bool) bool {
	checkIDs, err := q.ListAffectedCheckIDsForIncident(ctx, incidentID)
	if err != nil {
		return false
	}
	for _, cid := range checkIDs {
		c, err := q.GetCheck(ctx, cid)
		if err != nil {
			continue
		}
		if c.ComponentID.Valid && pageComponentIDs[c.ComponentID.Int64] {
			return true
		}
	}
	return false
}

// --- Overall status computation ---

func faviconForStatus(overall string) string {
	switch overall {
	case "operational", "degraded", "maintenance":
		return overall
	case "partial_outage", "major_outage":
		return "outage"
	default:
		return "unknown"
	}
}

func computeOverall(components []componentView, incidents []incidentView, maint []maintenanceView) string {
	if len(components) == 0 && len(incidents) == 0 && len(maint) == 0 {
		return "operational"
	}
	var down, degraded, total, underMaint int
	for _, c := range components {
		total++
		switch c.Status {
		case "down":
			down++
		case "degraded":
			degraded++
		case "maintenance":
			underMaint++
		}
	}
	// Active maintenance wins over "operational" but not over incidents.
	if len(incidents) > 0 {
		if total > 0 && down*2 > total {
			return "major_outage"
		}
		if down > 0 {
			return "partial_outage"
		}
		if degraded > 0 {
			return "degraded"
		}
		return "partial_outage"
	}
	if down > 0 {
		if total > 0 && down*2 > total {
			return "major_outage"
		}
		return "partial_outage"
	}
	if degraded > 0 {
		return "degraded"
	}
	if underMaint > 0 || hasInProgressMaintenance(maint) {
		return "maintenance"
	}
	return "operational"
}

func hasInProgressMaintenance(views []maintenanceView) bool {
	for _, v := range views {
		if v.InProgress {
			return true
		}
	}
	return false
}

// --- Unlock cookie / HMAC ---

func unlockCookieName(pageID int64) string {
	return fmt.Sprintf("%s%d", unlockCookiePrefix, pageID)
}

func (h *Handler) signUnlock(page store.StatusPage) string {
	mac := hmac.New(sha256.New, h.signingKey)
	fmt.Fprintf(mac, "%d|%s", page.ID, page.PasswordHash.String)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *Handler) validateUnlockCookie(r *http.Request, page store.StatusPage) bool {
	cookie, err := r.Cookie(unlockCookieName(page.ID))
	if err != nil {
		return false
	}
	expected := h.signUnlock(page)
	return hmac.Equal([]byte(cookie.Value), []byte(expected))
}

// --- Rate limiter (tiny, in-memory, per IP) ---

type unlockLimiter struct {
	mu      sync.Mutex
	entries map[string]*unlockEntry
}

type unlockEntry struct {
	fails      int
	firstFail  time.Time
	lockedTill time.Time
}

func newUnlockLimiter() *unlockLimiter {
	return &unlockLimiter{entries: make(map[string]*unlockEntry)}
}

func (u *unlockLimiter) allow(ip string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	e, ok := u.entries[ip]
	if !ok {
		return true
	}
	if now.Before(e.lockedTill) {
		return false
	}
	if now.Sub(e.firstFail) > unlockWindow {
		delete(u.entries, ip)
	}
	return true
}

func (u *unlockLimiter) recordFail(ip string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	e, ok := u.entries[ip]
	if !ok || now.Sub(e.firstFail) > unlockWindow {
		u.entries[ip] = &unlockEntry{fails: 1, firstFail: now}
		return
	}
	e.fails++
	if e.fails >= maxUnlockFails {
		e.lockedTill = now.Add(unlockLockoutFor)
	}
}

func (u *unlockLimiter) recordSuccess(ip string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.entries, ip)
}

// --- Misc ---

func (h *Handler) writeSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: https:; style-src 'self'; script-src 'self'; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), interest-cohort=()")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func preferredLang(accept string) string {
	if accept == "" {
		return "en"
	}
	if i := strings.IndexAny(accept, ",;"); i > 0 {
		accept = accept[:i]
	}
	accept = strings.TrimSpace(accept)
	if accept == "" {
		return "en"
	}
	if len(accept) > 10 {
		accept = accept[:10]
	}
	return accept
}

func anyIntersects(ids []int64, set map[int64]bool) bool {
	if len(set) == 0 {
		return true
	}
	for _, id := range ids {
		if set[id] {
			return true
		}
	}
	return false
}

func simplifyIncidents(incs []store.Incident) []map[string]any {
	out := make([]map[string]any, 0, len(incs))
	for _, inc := range incs {
		m := map[string]any{
			"id":         inc.ID,
			"title":      inc.Title,
			"severity":   inc.Severity,
			"status":     inc.Status,
			"started_at": inc.StartedAt.UTC().Format(time.RFC3339),
		}
		if inc.ResolvedAt.Valid {
			m["resolved_at"] = inc.ResolvedAt.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, m)
	}
	return out
}

