// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/component"
	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/maintenance"
	"github.com/TRC-Loop/cairn/internal/store"
	"github.com/go-chi/chi/v5"
)

const testEncryptionKey = "this-is-a-test-key-at-least-32-bytes-long!!"

func newTestHandler(t *testing.T) (*Handler, *Service, *store.Queries) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewService(db, q, logger)
	componentSvc := component.NewService(db, q, logger)
	maintenanceSvc := maintenance.NewService(db, q, logger)
	incidentSvc := incident.NewService(db, q, logger, maintenanceSvc)
	h := NewHandler(svc, componentSvc, maintenanceSvc, incidentSvc, q, logger, testEncryptionKey, nil, false)
	return h, svc, q
}

func newTestRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.ServeDefault)
	r.Route("/p/{slug}", func(r chi.Router) {
		r.Get("/", h.ServeBySlug)
		r.Post("/unlock", h.HandleUnlock)
		r.Get("/api.json", h.ServeJSON)
	})
	return r
}

func TestHandlerServes404ForUnknownSlug(t *testing.T) {
	h, _, _ := newTestHandler(t)
	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/nonexistent/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "No status page here") {
		t.Errorf("expected 404 body content, got %q", string(body))
	}
}

func TestHandlerRendersStatusPage(t *testing.T) {
	h, svc, q := newTestHandler(t)
	ctx := context.Background()

	page, err := svc.Create(ctx, CreateInput{
		Slug:        "main",
		Title:       "Cairn Demo Status",
		Description: "Monitoring for demo services.",
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	comp := createComponent(t, q, "Website")
	if err := svc.AddComponent(ctx, page.ID, comp.ID, 0); err != nil {
		t.Fatalf("add component: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/main/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	b := string(body)
	for _, needle := range []string{"Cairn Demo Status", "Website", "body-dark"} {
		if !strings.Contains(b, needle) {
			t.Errorf("expected body to contain %q", needle)
		}
	}
	if strings.Contains(b, "googleapis") || strings.Contains(b, "cdn.") {
		t.Errorf("rendered HTML must not reference external CDNs")
	}

	resp2, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on default route, got %d", resp2.StatusCode)
	}
	b2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(b2), "Cairn Demo Status") {
		t.Errorf("default route should render the default page")
	}
}

func TestHandlerRendersUnlockWhenPasswordSet(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "private", Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.SetPassword(ctx, page.ID, "hunter2"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/private/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "This status page is private") {
		t.Errorf("expected unlock page, got %q", string(body))
	}
}

func TestUnlockHandlerCorrectPasswordSetsCookieAllowsAccess(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "private", Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.SetPassword(ctx, page.ID, "hunter2"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{}
	form.Set("password", "hunter2")
	resp, err := client.PostForm(srv.URL+"/p/private/unlock", form)
	if err != nil {
		t.Fatalf("POST unlock: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", resp.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if strings.HasPrefix(c.Name, unlockCookiePrefix) {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatalf("expected unlock cookie; got %v", resp.Cookies())
	}
	if !cookie.HttpOnly {
		t.Error("unlock cookie must be HttpOnly")
	}

	req, _ := http.NewRequest("GET", srv.URL+"/p/private/", nil)
	req.AddCookie(cookie)
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET with cookie: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with cookie, got %d", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if strings.Contains(string(body), "This status page is private") {
		t.Errorf("unlock page still showing despite valid cookie")
	}
}

func TestUnlockHandlerWrongPasswordRendersError(t *testing.T) {
	h, svc, _ := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "private", Title: "Private"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.SetPassword(ctx, page.ID, "hunter2"); err != nil {
		t.Fatalf("set password: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	form := url.Values{}
	form.Set("password", "wrong")
	resp, err := http.PostForm(srv.URL+"/p/private/unlock", form)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Incorrect password") {
		t.Errorf("expected error message, got %q", string(body))
	}
}

func TestJSONEndpointAllUpReturnsOperational(t *testing.T) {
	h, svc, q := newTestHandler(t)
	ctx := context.Background()
	page, err := svc.Create(ctx, CreateInput{Slug: "main", Title: "Main"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	comp := createComponent(t, q, "API")
	c := createCheckFor(t, q, comp.ID, "http-api")

	if err := q.UpdateCheckStatus(ctx, store.UpdateCheckStatusParams{
		LastStatus: "up",
		ID:         c.ID,
	}); err != nil {
		t.Fatalf("set status: %v", err)
	}
	if err := svc.AddComponent(ctx, page.ID, comp.ID, 0); err != nil {
		t.Fatalf("add: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/main/api.json")
	if err != nil {
		t.Fatalf("GET json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		Page struct {
			Title string `json:"title"`
			Slug  string `json:"slug"`
		} `json:"page"`
		OverallStatus string `json:"overall_status"`
		Components    []struct {
			ID     int64  `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"components"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Page.Title != "Main" {
		t.Errorf("expected title Main, got %q", out.Page.Title)
	}
	if out.Page.Slug != "main" {
		t.Errorf("expected slug main, got %q", out.Page.Slug)
	}
	if out.OverallStatus != "operational" {
		t.Errorf("expected operational, got %q", out.OverallStatus)
	}
	if len(out.Components) != 1 || out.Components[0].Status != "up" {
		t.Errorf("expected one up component, got %+v", out.Components)
	}
}

func TestIntegrationGETRendersHTML(t *testing.T) {
	h, svc, q := newTestHandler(t)
	ctx := context.Background()

	page, err := svc.Create(ctx, CreateInput{Slug: "integration", Title: "Integration Demo"})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	comp := createComponent(t, q, "API")
	c := createCheckFor(t, q, comp.ID, "http-api")

	// Insert a few daily aggregates spanning the last few days.
	today := startOfDayUTC(time.Now().UTC())
	upsertDaily(t, q, c.ID, today, 100, 0, 0)
	upsertDaily(t, q, c.ID, today.AddDate(0, 0, -1), 100, 0, 0)
	upsertDaily(t, q, c.ID, today.AddDate(0, 0, -2), 50, 0, 50)

	if err := svc.AddComponent(ctx, page.ID, comp.ID, 0); err != nil {
		t.Fatalf("add: %v", err)
	}

	srv := httptest.NewServer(newTestRouter(h))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/p/integration/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	b := string(body)
	for _, needle := range []string{"Integration Demo", "API", "component-list", "history-bar", "tick"} {
		if !strings.Contains(b, needle) {
			t.Errorf("expected marker %q in rendered HTML", needle)
		}
	}
	if got := resp.Header.Get("Content-Security-Policy"); got == "" {
		t.Errorf("expected CSP header to be set")
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("expected X-Content-Type-Options nosniff, got %q", got)
	}
}
