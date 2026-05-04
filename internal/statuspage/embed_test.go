// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newEmbedRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/p/{slug}", func(r chi.Router) {
		r.Get("/embed.js", h.ServeEmbedScript)
		r.Get("/embed", h.ServeEmbed)
	})
	return r
}

func TestEmbedScriptIncludesIframeSrcFromHost(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{Slug: "main", Title: "Main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/main/embed.js")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("expected JS content-type, got %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if !strings.Contains(bs, srv.URL+"/p/main/embed") {
		t.Errorf("expected script to contain iframe src %q, got:\n%s", srv.URL+"/p/main/embed", bs)
	}
}

func TestEmbedScriptHonorsXForwardedProto(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{Slug: "main", Title: "Main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	ts := httptest.NewServer(newEmbedRouter(h))
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/p/main/embed.js", nil)
	req.Host = "status.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "https://status.example.com/p/main/embed") {
		t.Errorf("expected https URL with forwarded host, got:\n%s", string(body))
	}
}

func TestEmbedSizesDistinguishable(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{Slug: "main", Title: "Main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	cases := []struct {
		size   string
		marker string
	}{
		{"full", "full-wrap"},
		{"compact", "compact-wrap"},
		{"badge", "badge badge-"},
	}
	for _, tc := range cases {
		t.Run(tc.size, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/p/main/embed?size=" + tc.size)
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tc.marker) {
				t.Errorf("expected marker %q for size=%s, got:\n%s", tc.marker, tc.size, string(body))
			}
		})
	}
}

func TestEmbedInvalidSizeReturns400(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{Slug: "main", Title: "Main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/main/embed?size=bogus")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestEmbedPasswordProtectedReturns403(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "private", Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.SetPassword(ctx, page.ID, "hunter2"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/private/embed")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "private") {
		t.Errorf("expected private-page body, got:\n%s", string(body))
	}
}

func TestEmbedUnknownSlugReturns404(t *testing.T) {
	h, _, _ := newTestHandler(t)
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/ghost/embed")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	if !strings.Contains(bs, "Status page not found") {
		t.Errorf("expected embed 404 body, got:\n%s", bs)
	}
	// Must NOT be the main site's 404 template.
	if strings.Contains(bs, "No status page here") {
		t.Errorf("got main 404 template instead of embed 404")
	}
}

func TestEmbedCSPAllowsFrameAncestors(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{Slug: "main", Title: "Main"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	srv := httptest.NewServer(newEmbedRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/main/embed")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors *") {
		t.Errorf("expected CSP to allow frame-ancestors *, got %q", csp)
	}
	if xfo := resp.Header.Get("X-Frame-Options"); xfo != "" {
		t.Errorf("embed route must not set X-Frame-Options, got %q", xfo)
	}
}

func TestBadgeLabelsForEachOverallStatus(t *testing.T) {
	cases := map[string]string{
		"operational":    "All systems operational",
		"degraded":       "Degraded performance",
		"partial_outage": "Partial outage",
		"major_outage":   "Major outage",
		"maintenance":    "Scheduled maintenance",
	}
	for status, want := range cases {
		if got := overallLabel(status); got != want {
			t.Errorf("overallLabel(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		in    string
		want  string
		okExp bool
	}{
		{"", "full", true},
		{"full", "full", true},
		{"compact", "compact", true},
		{"badge", "badge", true},
		{"Badge", "", false},
		{"xl", "", false},
	}
	for _, tc := range cases {
		got, ok := parseSize(tc.in)
		if ok != tc.okExp || (ok && got != tc.want) {
			t.Errorf("parseSize(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.okExp)
		}
	}
}

func TestParseTheme(t *testing.T) {
	cases := map[string]string{
		"":        "auto",
		"auto":    "auto",
		"light":   "light",
		"dark":    "dark",
		"invalid": "auto",
	}
	for in, want := range cases {
		if got := parseTheme(in); got != want {
			t.Errorf("parseTheme(%q) = %q, want %q", in, got, want)
		}
	}
}
