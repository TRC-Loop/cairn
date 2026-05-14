// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// fetchPublicPage drives a status page through HTTP and returns the body,
// so the assertions run against the same template path real visitors hit.
func fetchPublicPage(t *testing.T, h *Handler, slug string) string {
	t.Helper()
	srv := httptest.NewServer(newTestRouter(h))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/p/" + slug + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}

func TestPublicPageHasNoInlineEventHandlers(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	page, err := svc.Create(context.Background(), CreateInput{
		Slug: "csp", Title: "CSP", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.UpdateFlags(context.Background(), page.ID, false, true); err != nil {
		t.Fatalf("flags: %v", err)
	}

	body := fetchPublicPage(t, h, "csp")
	re := regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
	if m := re.FindString(body); m != "" {
		t.Fatalf("inline event handler %q would be blocked by CSP", strings.TrimSpace(m))
	}
}

func TestPublicPageNoDialogInsideList(t *testing.T) {
	h, svc, q := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "dlg", Title: "Dlg", IsDefault: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	comp := createComponent(t, q, "Web")
	createCheckFor(t, q, comp.ID, "http-web")
	if err := svc.AddComponent(ctx, page.ID, comp.ID, 0); err != nil {
		t.Fatalf("add component: %v", err)
	}
	if err := svc.SetComponentShowMonitors(ctx, page.ID, comp.ID, ShowMonitorsDefaultClosed); err != nil {
		t.Fatalf("set show_monitors: %v", err)
	}

	body := fetchPublicPage(t, h, "dlg")

	// Invalid nesting: <dialog> as direct or nested child of <ul> causes
	// browsers to relocate it, which broke the monitors modal and footer.
	listOpens := indexAll(body, "<ul")
	listCloses := indexAll(body, "</ul>")
	dialogOpens := indexAll(body, "<dialog")
	for _, d := range dialogOpens {
		depth := 0
		for _, o := range listOpens {
			if o < d {
				depth++
			}
		}
		for _, c := range listCloses {
			if c < d {
				depth--
			}
		}
		if depth > 0 {
			t.Fatalf("<dialog> at offset %d is inside an open <ul>; browsers will relocate it", d)
		}
	}
}

func TestPublicPageFooterAlwaysRendered(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	page, err := svc.Create(context.Background(), CreateInput{
		Slug: "ft", Title: "Ft", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.UpdateFlags(context.Background(), page.ID, false, true); err != nil {
		t.Fatalf("flags: %v", err)
	}
	body := fetchPublicPage(t, h, "ft")
	if !strings.Contains(body, "<footer") {
		t.Fatal("footer missing when show_history is enabled")
	}
	if !strings.Contains(body, "</footer>") {
		t.Fatal("footer not closed")
	}
}

func TestPublicPageHistoryButtonLabel(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	page, err := svc.Create(context.Background(), CreateInput{
		Slug: "hb", Title: "Hb", IsDefault: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.UpdateFlags(context.Background(), page.ID, false, true); err != nil {
		t.Fatalf("flags: %v", err)
	}
	body := fetchPublicPage(t, h, "hb")
	if !strings.Contains(body, `data-action="open-history"`) {
		t.Fatal("history trigger missing")
	}
	if !regexp.MustCompile(`(?s)data-action="open-history"[^>]*>\s*History\s*<`).MatchString(body) {
		t.Fatal("history button should be labelled 'History'")
	}
}

func TestMonitorDialogIdMatchesComponentTrigger(t *testing.T) {
	h, svc, q := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "match", Title: "Match", IsDefault: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	comp := createComponent(t, q, "Web")
	createCheckFor(t, q, comp.ID, "http-web")
	if err := svc.AddComponent(ctx, page.ID, comp.ID, 0); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := svc.SetComponentShowMonitors(ctx, page.ID, comp.ID, ShowMonitorsDefaultClosed); err != nil {
		t.Fatalf("set show_monitors: %v", err)
	}

	body := fetchPublicPage(t, h, "match")
	openers := regexp.MustCompile(`data-monitors-open="(\d+)"`).FindAllStringSubmatch(body, -1)
	if len(openers) == 0 {
		t.Fatal("no data-monitors-open attributes; nothing would open")
	}
	for _, m := range openers {
		needle := `data-monitors-dialog="` + m[1] + `"`
		if !strings.Contains(body, needle) {
			t.Errorf("trigger for component %s has no matching dialog", m[1])
		}
	}
}

func TestStaticAssetsCacheBusted(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	if _, err := svc.Create(context.Background(), CreateInput{
		Slug: "cb", Title: "Cb", IsDefault: true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	body := fetchPublicPage(t, h, "cb")
	v := AssetVersion()
	if v == "" {
		t.Fatal("AssetVersion empty")
	}
	for _, needle := range []string{
		"/static/css/status.css?v=" + v,
		"/static/js/status.js?v=" + v,
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("asset reference %q missing; cached browsers will keep old JS/CSS", needle)
		}
	}
}

func indexAll(s, sub string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(s[i:], sub)
		if j < 0 {
			return out
		}
		out = append(out, i+j)
		i += j + len(sub)
	}
}
