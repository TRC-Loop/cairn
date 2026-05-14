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
	"sync"
	"testing"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/store"
)

type captureNotifier struct {
	mu     sync.Mutex
	events []string
}

func (c *captureNotifier) NotifyChecks(_ context.Context, eventType string, _ int64, _ incident.NotifierPayload, _ []int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, eventType)
	return 1, nil
}

func (c *captureNotifier) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.events))
	copy(out, c.events)
	return out
}

func newIncidentsTestServer(t *testing.T) (*httptest.Server, *store.Queries, *incident.Service, *captureNotifier) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	svc := incident.NewService(db, q, logger, nil)
	cap := &captureNotifier{}
	svc.SetNotifier(cap)
	incidentsH := NewIncidentsHandler(q, svc, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, nil, nil, nil, incidentsH, nil, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q, svc, cap
}

func seedCheck(t *testing.T, q *store.Queries, name string) store.Check {
	t.Helper()
	c, err := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: name, Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{"url":"https://x"}`,
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}
	return c
}

func seedManualIncident(t *testing.T, svc *incident.Service, checkIDs []int64) store.Incident {
	t.Helper()
	in, err := svc.CreateManual(context.Background(), incident.CreateIncidentInput{
		Title:            "Seed",
		Severity:         incident.SeverityMinor,
		InitialMessage:   "Seed message",
		AffectedCheckIDs: checkIDs,
		CreatedByUserID:  1,
	})
	if err != nil {
		t.Fatalf("seed incident: %v", err)
	}
	return in
}

func TestIncidentsCreateValidation(t *testing.T) {
	srv, q, _, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")

	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents", map[string]any{
		"title": "X", "severity": "major", "initial_message": "msg", "affected_check_ids": []int64{},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty checks, got %d", resp.StatusCode)
	}
}

func TestIncidentsCreateRejectsInvalidCheckIDs(t *testing.T) {
	srv, q, _, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents", map[string]any{
		"title": "X", "severity": "major", "initial_message": "m", "affected_check_ids": []int64{99999},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIncidentsCreateSuccessAndNotifies(t *testing.T) {
	srv, q, _, cap := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "web")
	client := loginAs(t, srv, "admin", "password-long-enough")

	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents", map[string]any{
		"title": "Outage", "severity": "major", "initial_message": "Looking into it.",
		"affected_check_ids": []int64{c.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	in, _ := out["incident"].(map[string]any)
	if in["title"] != "Outage" {
		t.Fatalf("title mismatch: %v", in)
	}
	updates, _ := out["updates"].([]any)
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if got := cap.seen(); len(got) != 1 || got[0] != incident.EventIncidentOpened {
		t.Fatalf("expected incident_opened, got %v", got)
	}
}

func TestIncidentsListFilters(t *testing.T) {
	srv, q, svc, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "x")
	in := seedManualIncident(t, svc, []int64{c.ID})
	// resolve via Transition
	if err := svc.Transition(context.Background(), in.ID, incident.StatusResolved, nil, "done"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	in2 := seedManualIncident(t, svc, []int64{c.ID})
	_ = in2

	client := loginAs(t, srv, "admin", "password-long-enough")
	for _, tc := range []struct {
		status string
		want   int
	}{
		{"all", 2},
		{"active", 1},
		{"resolved", 1},
	} {
		resp := doJSON(t, client, srv, http.MethodGet, "/api/incidents?status="+tc.status, nil)
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		incs, _ := out["incidents"].([]any)
		if len(incs) != tc.want {
			t.Fatalf("status=%s: expected %d, got %d", tc.status, tc.want, len(incs))
		}
	}
}

func TestIncidentsDetail(t *testing.T) {
	srv, q, svc, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "y")
	in := seedManualIncident(t, svc, []int64{c.ID})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodGet, "/api/incidents/"+intStr(in.ID), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	updates, _ := out["updates"].([]any)
	checks, _ := out["affected_checks"].([]any)
	if len(updates) != 1 || len(checks) != 1 {
		t.Fatalf("expected 1 update + 1 check, got %d/%d", len(updates), len(checks))
	}
}

func TestIncidentsAddUpdateTransitionAndNotifies(t *testing.T) {
	srv, q, svc, cap := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "z")
	in := seedManualIncident(t, svc, []int64{c.ID})
	cap.events = nil

	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents/"+intStr(in.ID)+"/updates", map[string]any{
		"message": "Identified", "new_status": "identified",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if got := cap.seen(); len(got) != 1 || got[0] != incident.EventIncidentUpdated {
		t.Fatalf("expected incident_updated, got %v", got)
	}
}

func TestIncidentsAddUpdateInvalidTransition(t *testing.T) {
	srv, q, svc, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "z2")
	in := seedManualIncident(t, svc, []int64{c.ID})
	if err := svc.Transition(context.Background(), in.ID, incident.StatusResolved, nil, ""); err != nil {
		t.Fatalf("transition: %v", err)
	}
	client := loginAs(t, srv, "admin", "password-long-enough")
	// resolved -> monitoring is invalid
	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents/"+intStr(in.ID)+"/updates", map[string]any{
		"message": "x", "new_status": "monitoring",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIncidentsResolveSetsResolvedAtAndNotifies(t *testing.T) {
	srv, q, svc, cap := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "rs")
	in := seedManualIncident(t, svc, []int64{c.ID})
	cap.events = nil
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents/"+intStr(in.ID)+"/updates", map[string]any{
		"message": "Resolved.", "new_status": "resolved",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	got, err := q.GetIncident(context.Background(), in.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.ResolvedAt.Valid {
		t.Fatal("resolved_at should be set")
	}
	if events := cap.seen(); len(events) != 1 || events[0] != incident.EventIncidentResolved {
		t.Fatalf("expected incident_resolved, got %v", events)
	}
}

func TestIncidentsAffectedChecksAddRemove(t *testing.T) {
	srv, q, svc, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c1 := seedCheck(t, q, "a")
	c2 := seedCheck(t, q, "b")
	in := seedManualIncident(t, svc, []int64{c1.ID})
	client := loginAs(t, srv, "admin", "password-long-enough")

	resp := doJSON(t, client, srv, http.MethodPost, "/api/incidents/"+intStr(in.ID)+"/affected-checks", map[string]any{"check_id": c2.ID})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	ids, _ := q.ListAffectedCheckIDsForIncident(context.Background(), in.ID)
	if len(ids) != 2 {
		t.Fatalf("expected 2 affected checks, got %d", len(ids))
	}
	resp = doJSON(t, client, srv, http.MethodDelete, "/api/incidents/"+intStr(in.ID)+"/affected-checks/"+intStr(c2.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d", resp.StatusCode)
	}
	ids, _ = q.ListAffectedCheckIDsForIncident(context.Background(), in.ID)
	if len(ids) != 1 {
		t.Fatalf("expected 1, got %d", len(ids))
	}
}

func TestIncidentsDeleteCascades(t *testing.T) {
	srv, q, svc, _ := newIncidentsTestServer(t)
	seedUser(t, q, "admin", "password-long-enough", "admin")
	c := seedCheck(t, q, "z3")
	in := seedManualIncident(t, svc, []int64{c.ID})
	client := loginAs(t, srv, "admin", "password-long-enough")
	resp := doJSON(t, client, srv, http.MethodDelete, "/api/incidents/"+intStr(in.ID), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if _, err := q.GetIncident(context.Background(), in.ID); err == nil {
		t.Fatal("incident should be gone")
	} else if err != sql.ErrNoRows {
		t.Fatalf("unexpected err: %v", err)
	}
	ids, _ := q.ListAffectedCheckIDsForIncident(context.Background(), in.ID)
	if len(ids) != 0 {
		t.Fatal("affected check links should cascade")
	}
	updates, _ := q.ListUpdatesForIncident(context.Background(), in.ID)
	if len(updates) != 0 {
		t.Fatal("updates should cascade")
	}
}

func intStr(n int64) string { return strconv.FormatInt(n, 10) }
