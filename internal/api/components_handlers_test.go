// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/component"
	"github.com/TRC-Loop/cairn/internal/store"
)

func newComponentsTestServer(t *testing.T) (*httptest.Server, *store.Queries, *sql.DB) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	checksH := NewChecksHandler(q, db, logger)
	componentSvc := component.NewService(db, q, logger)
	componentsH := NewComponentsHandler(q, componentSvc, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, checksH, componentsH, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q, db
}

func TestComponentsListUnauthenticated(t *testing.T) {
	srv, _, _ := newComponentsTestServer(t)
	resp, _ := http.Get(srv.URL + "/api/components")
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestComponentsListEmpty(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/components", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	comps, _ := out["components"].([]any)
	if len(comps) != 0 {
		t.Fatalf("expected 0 components, got %d", len(comps))
	}
}

func TestComponentsCreate(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/components", map[string]any{
		"name":          "API",
		"description":   "Public API",
		"display_order": 1,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	c, _ := out["component"].(map[string]any)
	if c["name"] != "API" {
		t.Fatalf("unexpected: %v", out)
	}
}

func TestComponentsCreateMissingName(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/components", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestComponentsCreateAsViewerForbidden(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "viewer", "password-long-enough", "viewer")
	client := loginAs(t, srv, "viewer", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/components", map[string]any{"name": "X"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestComponentsGetWithChecks(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	c, err := q.CreateComponent(ctx, store.CreateComponentParams{Name: "API", DisplayOrder: 0})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	_, err = q.CreateCheck(ctx, store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{"url":"https://x"}`,
		ComponentID: sql.NullInt64{Int64: c.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/components/"+itoa64(c.ID), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	checks, _ := out["checks"].([]any)
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
}

func TestComponentsUpdate(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c, _ := q.CreateComponent(context.Background(), store.CreateComponentParams{Name: "Old", DisplayOrder: 0})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/components/"+itoa64(c.ID), map[string]any{"name": "New"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	updated, _ := q.GetComponent(context.Background(), c.ID)
	if updated.Name != "New" {
		t.Fatalf("expected New, got %s", updated.Name)
	}
}

func TestComponentsDeleteUnsetsCheckComponent(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	ctx := context.Background()
	c, _ := q.CreateComponent(ctx, store.CreateComponentParams{Name: "A", DisplayOrder: 0})
	ch, _ := q.CreateCheck(ctx, store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{"url":"https://x"}`,
		ComponentID: sql.NullInt64{Int64: c.ID, Valid: true},
	})

	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodDelete, "/api/components/"+itoa64(c.ID), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	got, err := q.GetCheck(ctx, ch.ID)
	if err != nil {
		t.Fatalf("get check: %v", err)
	}
	if got.ComponentID.Valid {
		t.Fatalf("expected component_id null after delete, got %v", got.ComponentID.Int64)
	}
}

func TestComponentsReorder(t *testing.T) {
	srv, q, _ := newComponentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c, _ := q.CreateComponent(context.Background(), store.CreateComponentParams{Name: "A", DisplayOrder: 0})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/components/"+itoa64(c.ID)+"/reorder", map[string]any{"display_order": 5})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	got, _ := q.GetComponent(context.Background(), c.ID)
	if got.DisplayOrder != 5 {
		t.Fatalf("expected 5, got %d", got.DisplayOrder)
	}
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}
