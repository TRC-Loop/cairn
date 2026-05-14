// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/statuspage"
	"github.com/TRC-Loop/cairn/internal/store"
)

func newStatusPagesTestServer(t *testing.T) (*httptest.Server, *store.Queries) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	svc := statuspage.NewService(db, q, logger)
	pagesH := NewStatusPagesHandler(q, svc, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, pagesH, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q
}

func TestStatusPagesUnauthenticated(t *testing.T) {
	srv, _ := newStatusPagesTestServer(t)
	resp, err := http.Get(srv.URL + "/api/status-pages")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestStatusPagesCreate(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages", map[string]any{
		"slug":  "main",
		"title": "Main Status",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
}

func TestStatusPagesCreateBadSlug(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages", map[string]any{
		"slug":  "Bad Slug!",
		"title": "X",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStatusPagesCreateDuplicateSlug(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	_, _ = q.CreateStatusPage(context.Background(), store.CreateStatusPageParams{Slug: "dup", Title: "T"})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages", map[string]any{
		"slug":  "dup",
		"title": "T",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestStatusPagesCreateAsViewerForbidden(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "viewer", "password-long-enough", "viewer")
	client := loginAs(t, srv, "viewer", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages", map[string]any{"slug": "x", "title": "T"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestStatusPagesGetWithComponents(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	p, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "x", Title: "T"})
	c, _ := q.CreateComponent(ctx, store.CreateComponentParams{Name: "API", DisplayOrder: 0})
	_ = q.AddComponentToStatusPage(ctx, store.AddComponentToStatusPageParams{
		StatusPageID: p.ID, ComponentID: c.ID, DisplayOrder: 0,
	})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/status-pages/"+itoa64(p.ID), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	comps, _ := out["components"].([]any)
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comps))
	}
}

func TestStatusPagesSetDefault(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	a, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "a", Title: "A", IsDefault: true})
	b, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "b", Title: "B"})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages/"+itoa64(b.ID)+"/default", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	got, _ := q.GetStatusPage(ctx, b.ID)
	if !got.IsDefault {
		t.Fatalf("expected b to be default")
	}
	gotA, _ := q.GetStatusPage(ctx, a.ID)
	if gotA.IsDefault {
		t.Fatalf("expected a no longer default")
	}
}

func TestStatusPagesSetPasswordAndClear(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	p, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "x", Title: "T"})
	client := loginAs(t, srv, "admin", "password-long-enough")

	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages/"+itoa64(p.ID)+"/password", map[string]any{"password": "secret-long"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	got, _ := q.GetStatusPage(ctx, p.ID)
	if !got.PasswordHash.Valid || got.PasswordHash.String == "" {
		t.Fatalf("expected password hash set")
	}

	resp2 := doJSON(t, client, srv, http.MethodPost, "/api/status-pages/"+itoa64(p.ID)+"/password", map[string]any{"password": ""})
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp2.StatusCode)
	}
	got2, _ := q.GetStatusPage(ctx, p.ID)
	if got2.PasswordHash.Valid && got2.PasswordHash.String != "" {
		t.Fatalf("expected password cleared")
	}
}

func TestStatusPagesSetPasswordTooShort(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	p, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "x", Title: "T"})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/status-pages/"+itoa64(p.ID)+"/password", map[string]any{"password": "short"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestStatusPagesSetComponentsReplaces(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	p, _ := q.CreateStatusPage(ctx, store.CreateStatusPageParams{Slug: "x", Title: "T"})
	c1, _ := q.CreateComponent(ctx, store.CreateComponentParams{Name: "A", DisplayOrder: 0})
	c2, _ := q.CreateComponent(ctx, store.CreateComponentParams{Name: "B", DisplayOrder: 0})
	c3, _ := q.CreateComponent(ctx, store.CreateComponentParams{Name: "C", DisplayOrder: 0})
	_ = q.AddComponentToStatusPage(ctx, store.AddComponentToStatusPageParams{StatusPageID: p.ID, ComponentID: c1.ID, DisplayOrder: 0})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPut, "/api/status-pages/"+itoa64(p.ID)+"/components", map[string]any{
		"component_ids": []int64{c3.ID, c2.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	got, _ := q.ListComponentsForStatusPage(ctx, p.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 components, got %d", len(got))
	}
	if got[0].ID != c3.ID || got[1].ID != c2.ID {
		t.Fatalf("expected order [c3, c2], got [%d, %d]", got[0].ID, got[1].ID)
	}
}

func TestStatusPagesUpdate(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	p, _ := q.CreateStatusPage(context.Background(), store.CreateStatusPageParams{Slug: "x", Title: "Old"})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/status-pages/"+itoa64(p.ID), map[string]any{"title": "New"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	got, _ := q.GetStatusPage(context.Background(), p.ID)
	if got.Title != "New" {
		t.Fatalf("expected New, got %s", got.Title)
	}
}

func TestStatusPagesDelete(t *testing.T) {
	srv, q := newStatusPagesTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	p, _ := q.CreateStatusPage(context.Background(), store.CreateStatusPageParams{Slug: "x", Title: "T"})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodDelete, "/api/status-pages/"+itoa64(p.ID), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}
