// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// embedCSS is the inlined stylesheet served with every iframe document.
// Reading at init keeps the hot path allocation-free.
var embedCSS = mustLoadEmbedCSS()

func mustLoadEmbedCSS() string {
	b, err := fs.ReadFile(staticFS, "static/css/embed.css")
	if err != nil {
		panic("statuspage: embed.css missing: " + err.Error())
	}
	return string(b)
}

type embedView struct {
	PageTitle         string
	Slug              string
	Lang              string
	Theme             string
	EmbedCSS          string
	OverallStatus     string
	OverallStatusFavicon string
	Components        []componentView
	ActiveIncidents   []incidentView
	ActiveMaintenance []maintenanceView
	FooterHTML        template.HTML
}

// ServeEmbed renders the iframe body for an embeddable status widget.
// Size chosen via ?size={full|compact|badge}; unknown sizes are a 400.
// Password-protected pages return 403 — the iframe can't host a login form.
func (h *Handler) ServeEmbed(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	size, ok := parseSize(r.URL.Query().Get("size"))
	if !ok {
		h.writeEmbedHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid size"))
		return
	}
	theme := parseTheme(r.URL.Query().Get("theme"))

	page, err := h.service.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			h.renderEmbedMessage(w, r, slug, theme, "embed_404.html", http.StatusNotFound)
			return
		}
		h.logger.Error("embed get page failed", "slug", slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if page.PasswordHash.Valid && page.PasswordHash.String != "" {
		h.renderEmbedMessage(w, r, slug, theme, "embed_private.html", http.StatusForbidden)
		return
	}

	view, err := h.buildPageView(r.Context(), page, r.Header.Get("Accept-Language"))
	if err != nil {
		h.logger.Error("embed build view failed", "slug", slug, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	ev := &embedView{
		PageTitle:         page.Title,
		Slug:              page.Slug,
		Lang:              view.Lang,
		Theme:             theme,
		EmbedCSS:          embedCSS,
		OverallStatus:     view.OverallStatus,
		OverallStatusFavicon: view.OverallStatusFavicon,
		Components:        view.Components,
		ActiveIncidents:   view.ActiveIncidents,
		ActiveMaintenance: view.ActiveMaintenance,
		FooterHTML:        view.FooterHTML,
	}

	h.writeEmbedHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	if err := h.templates.ExecuteTemplate(w, templateForSize(size), ev); err != nil {
		h.logger.Error("embed render failed", "slug", slug, "size", size, "err", err)
	}
}

// ServeEmbedScript returns the loader JS operators paste into their site.
// The iframe URL is derived from the incoming request — host + forwarded
// proto — so the snippet works without operator-supplied config.
func (h *Handler) ServeEmbedScript(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	base := baseURL(r)
	iframeSrc := fmt.Sprintf("%s/p/%s/embed", base, slug)

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = fmt.Fprintf(w, loaderJSTemplate, jsString(iframeSrc))
}

func (h *Handler) renderEmbedMessage(w http.ResponseWriter, r *http.Request, slug, theme, tmpl string, status int) {
	ev := &embedView{
		PageTitle:     "Status",
		Slug:          slug,
		Lang:          preferredLang(r.Header.Get("Accept-Language")),
		Theme:         theme,
		EmbedCSS:      embedCSS,
		OverallStatus: "unknown",
		OverallStatusFavicon: faviconForStatus("unknown"),
	}
	h.writeEmbedHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, tmpl, ev); err != nil {
		h.logger.Error("embed message render failed", "tmpl", tmpl, "err", err)
	}
}

// writeEmbedHeaders is the embed-flavored security header set. X-Frame-Options
// is explicitly absent and the CSP uses frame-ancestors * so operators can
// embed on any origin.
func (h *Handler) writeEmbedHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; font-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors *; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), interest-cohort=()")
	w.Header().Del("X-Frame-Options")
}

func parseSize(s string) (string, bool) {
	switch s {
	case "", "full":
		return "full", true
	case "compact", "badge":
		return s, true
	}
	return "", false
}

func parseTheme(s string) string {
	switch s {
	case "light", "dark", "auto":
		return s
	}
	return "auto"
}

func templateForSize(size string) string {
	switch size {
	case "compact":
		return "embed_compact.html"
	case "badge":
		return "embed_badge.html"
	}
	return "embed_full.html"
}

// baseURL returns scheme://host for the current request, honoring
// X-Forwarded-Proto when the server sits behind a reverse proxy.
func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
		if i := strings.IndexByte(fp, ','); i > 0 {
			fp = fp[:i]
		}
		fp = strings.TrimSpace(fp)
		if fp == "http" || fp == "https" {
			scheme = fp
		}
	}
	return scheme + "://" + r.Host
}

// jsString escapes a Go string for embedding inside a JS single-quoted literal.
// We only substitute characters that would break out of the string, not a
// full JSON encode, because the caller controls the content (a URL we built).
func jsString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '<':
			b.WriteString(`\u003c`)
		case '>':
			b.WriteString(`\u003e`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// loaderJSTemplate is inserted with the iframe URL at the first %s.
// Single %% escapes in format string only; none needed here, but keep in mind.
const loaderJSTemplate = `(function () {
  var base = '%s';
  var cs = document.currentScript;
  if (!cs) {
    var scripts = document.querySelectorAll('script[data-cairn-status]');
    cs = scripts[scripts.length - 1];
  }
  if (!cs) return;
  var size = cs.getAttribute('data-size') || 'full';
  var theme = cs.getAttribute('data-theme') || 'auto';
  var src = base + '?size=' + encodeURIComponent(size) + '&theme=' + encodeURIComponent(theme);
  var iframe = document.createElement('iframe');
  iframe.src = src;
  iframe.title = 'Cairn status widget';
  iframe.loading = 'lazy';
  iframe.scrolling = 'no';
  iframe.style.width = '100%%';
  iframe.style.border = '0';
  iframe.style.display = 'block';
  iframe.style.height = '0';
  cs.parentNode.insertBefore(iframe, cs);
  window.addEventListener('message', function (e) {
    if (!e.data || e.data.cairn !== true || e.data.type !== 'resize') return;
    if (e.source !== iframe.contentWindow) return;
    var h = parseInt(e.data.height, 10);
    if (isNaN(h) || h < 0 || h > 10000) return;
    iframe.style.height = h + 'px';
  });
})();
`

