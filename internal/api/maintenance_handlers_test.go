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
	"time"

	"github.com/TRC-Loop/cairn/internal/auth"
	"github.com/TRC-Loop/cairn/internal/maintenance"
	"github.com/TRC-Loop/cairn/internal/store"
)

type captureMaintNotifier struct {
	mu     sync.Mutex
	events []string
}

func (c *captureMaintNotifier) NotifyChecks(_ context.Context, eventType string, _ int64, _ maintenance.MaintenancePayload, _ []int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, eventType)
	return 1, nil
}

func (c *captureMaintNotifier) seen() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.events))
	copy(out, c.events)
	return out
}

func newMaintTestServer(t *testing.T) (*httptest.Server, *store.Queries, *maintenance.Service, *captureMaintNotifier) {
	t.Helper()
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sessionSvc := auth.NewSessionService(q, logger)
	authH := NewAuthHandler(q, sessionSvc, logger, false)
	svc := maintenance.NewService(db, q, logger)
	cap := &captureMaintNotifier{}
	svc.SetNotifier(cap)
	mh := NewMaintenanceHandler(q, svc, logger)
	srv := httptest.NewServer(NewRouter(logger, db, q, nil, nil, sessionSvc, authH, nil, nil, nil, nil, nil, nil, nil, mh, nil, nil, nil, nil, nil, nil, false, "dev", "unknown"))
	t.Cleanup(srv.Close)
	return srv, q, svc, cap
}

func seedComponent(t *testing.T, q *store.Queries, name string) store.Component {
	t.Helper()
	c, err := q.CreateComponent(context.Background(), store.CreateComponentParams{Name: name, DisplayOrder: 0})
	if err != nil {
		t.Fatalf("create component: %v", err)
	}
	return c
}

func adminClient(t *testing.T, srv *httptest.Server, q *store.Queries) *http.Client {
	t.Helper()
	seedUser(t, q, "admin", "password-long-enough", "admin")
	return loginAs(t, srv, "admin", "password-long-enough")
}

func decodeWindow(t *testing.T, resp *http.Response) maintenanceResponse {
	t.Helper()
	var body struct {
		Window maintenanceResponse `json:"window"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Window
}

func createWindow(t *testing.T, client *http.Client, srv *httptest.Server, q *store.Queries, startsIn, endsIn time.Duration) maintenanceResponse {
	t.Helper()
	c := seedComponent(t, q, "comp-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	now := time.Now().UTC()
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance", map[string]any{
		"title":                  "Test maint",
		"description":            "desc",
		"starts_at":              now.Add(startsIn).Format(time.RFC3339),
		"ends_at":                now.Add(endsIn).Format(time.RFC3339),
		"affected_component_ids": []int64{c.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create expected 201, got %d: %s", resp.StatusCode, string(b))
	}
	return decodeWindow(t, resp)
}

func TestMaintenanceCreateValidation(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	c := seedComponent(t, q, "api")
	now := time.Now().UTC()

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing title", map[string]any{
			"starts_at": now.Add(time.Hour).Format(time.RFC3339),
			"ends_at":   now.Add(2 * time.Hour).Format(time.RFC3339),
			"affected_component_ids": []int64{c.ID},
		}},
		{"ends before starts", map[string]any{
			"title":     "x",
			"starts_at": now.Add(2 * time.Hour).Format(time.RFC3339),
			"ends_at":   now.Add(time.Hour).Format(time.RFC3339),
			"affected_component_ids": []int64{c.ID},
		}},
		{"ends in past", map[string]any{
			"title":     "x",
			"starts_at": now.Add(-2 * time.Hour).Format(time.RFC3339),
			"ends_at":   now.Add(-time.Hour).Format(time.RFC3339),
			"affected_component_ids": []int64{c.ID},
		}},
		{"no components", map[string]any{
			"title":     "x",
			"starts_at": now.Add(time.Hour).Format(time.RFC3339),
			"ends_at":   now.Add(2 * time.Hour).Format(time.RFC3339),
			"affected_component_ids": []int64{},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance", tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestMaintenanceCreateStartsInPastIsInProgress(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)
	if w.State != maintenance.StateInProgress {
		t.Fatalf("expected in_progress, got %s", w.State)
	}
}

func TestMaintenanceCreateScheduled(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	if w.State != maintenance.StateScheduled {
		t.Fatalf("expected scheduled, got %s", w.State)
	}
}

func TestMaintenanceListFilters(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	createWindow(t, client, srv, q, time.Hour, 2*time.Hour)         // scheduled
	createWindow(t, client, srv, q, -10*time.Minute, time.Hour)     // in_progress

	resp := doJSON(t, client, srv, http.MethodGet, "/api/maintenance?upcoming=1", nil)
	defer resp.Body.Close()
	var body struct {
		Maintenance []maintenanceResponse `json:"maintenance"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Maintenance) != 1 || body.Maintenance[0].State != "scheduled" {
		t.Fatalf("expected 1 scheduled, got %+v", body.Maintenance)
	}

	resp2 := doJSON(t, client, srv, http.MethodGet, "/api/maintenance?status=in_progress", nil)
	defer resp2.Body.Close()
	var body2 struct {
		Maintenance []maintenanceResponse `json:"maintenance"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&body2)
	if len(body2.Maintenance) != 1 || body2.Maintenance[0].State != "in_progress" {
		t.Fatalf("expected 1 in_progress, got %+v", body2.Maintenance)
	}
}

func TestMaintenancePatchScheduledFields(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	now := time.Now().UTC()
	newStart := now.Add(3 * time.Hour).Format(time.RFC3339)
	newEnd := now.Add(4 * time.Hour).Format(time.RFC3339)
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"title":     "Updated",
		"starts_at": newStart,
		"ends_at":   newEnd,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}
	updated := decodeWindow(t, resp)
	if updated.Title != "Updated" {
		t.Fatalf("title not updated: %s", updated.Title)
	}
}

func TestMaintenancePatchInProgressRestricts(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)
	if w.State != maintenance.StateInProgress {
		t.Fatalf("setup: expected in_progress")
	}
	newStart := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"starts_at": newStart,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for starts_at change, got %d", resp.StatusCode)
	}

	// title + ends_at allowed
	newEnd := time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339)
	resp2 := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"title":   "Extended",
		"ends_at": newEnd,
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200, got %d: %s", resp2.StatusCode, string(b))
	}
}

func TestMaintenancePatchCompletedRejected(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)

	// End-now to make it completed
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w.ID, 10)+"/end-now", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("end-now expected 200, got %d", resp.StatusCode)
	}

	resp2 := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"title": "X",
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp2.StatusCode)
	}
}

func TestMaintenanceCancelScheduled(t *testing.T) {
	srv, q, _, cap := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w.ID, 10)+"/cancel", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := cap.seen(); len(got) != 0 {
		t.Fatalf("expected no notifications on cancel, got %v", got)
	}
}

func TestMaintenanceCancelInProgressRejected(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w.ID, 10)+"/cancel", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMaintenanceEndNowFiresNotification(t *testing.T) {
	srv, q, _, cap := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	// Create with starts_at in past so a check exists under that component is needed for notify
	c := seedComponent(t, q, "with-check")
	if _, err := q.CreateCheck(context.Background(), store.CreateCheckParams{
		Name: "x", Type: "http", Enabled: true, IntervalSeconds: 60, TimeoutSeconds: 10,
		FailureThreshold: 3, RecoveryThreshold: 1, ConfigJson: `{"url":"https://x"}`,
		ComponentID: sql.NullInt64{Int64: c.ID, Valid: true},
	}); err != nil {
		t.Fatalf("create check: %v", err)
	}
	now := time.Now().UTC()
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance", map[string]any{
		"title":                  "M",
		"starts_at":              now.Add(-5 * time.Minute).Format(time.RFC3339),
		"ends_at":                now.Add(time.Hour).Format(time.RFC3339),
		"affected_component_ids": []int64{c.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create expected 201, got %d", resp.StatusCode)
	}
	w := decodeWindow(t, resp)

	endResp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w.ID, 10)+"/end-now", nil)
	endResp.Body.Close()
	if endResp.StatusCode != http.StatusOK {
		t.Fatalf("end-now expected 200, got %d", endResp.StatusCode)
	}
	got := cap.seen()
	if len(got) != 1 || got[0] != maintenance.EventMaintenanceEnded {
		t.Fatalf("expected maintenance_ended notification, got %v", got)
	}
}

func TestMaintenanceEndNowOnScheduledRejected(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	resp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w.ID, 10)+"/end-now", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestMaintenanceDeleteRules(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)

	// scheduled: deletable
	w1 := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	resp := doJSON(t, client, srv, http.MethodDelete, "/api/maintenance/"+strconv.FormatInt(w1.ID, 10), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("scheduled delete expected 204, got %d", resp.StatusCode)
	}

	// in_progress: rejected
	w2 := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)
	resp2 := doJSON(t, client, srv, http.MethodDelete, "/api/maintenance/"+strconv.FormatInt(w2.ID, 10), nil)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("in_progress delete expected 400, got %d", resp2.StatusCode)
	}

	// end-now → completed → cannot delete
	endResp := doJSON(t, client, srv, http.MethodPost, "/api/maintenance/"+strconv.FormatInt(w2.ID, 10)+"/end-now", nil)
	endResp.Body.Close()
	resp3 := doJSON(t, client, srv, http.MethodDelete, "/api/maintenance/"+strconv.FormatInt(w2.ID, 10), nil)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Fatalf("completed delete expected 400, got %d", resp3.StatusCode)
	}
}

func TestMaintenancePatchScheduledAffectedComponents(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, time.Hour, 2*time.Hour)
	c2 := seedComponent(t, q, "alt")
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"affected_component_ids": []int64{c2.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}
	updated := decodeWindow(t, resp)
	if len(updated.AffectedComponentIDs) != 1 || updated.AffectedComponentIDs[0] != c2.ID {
		t.Fatalf("expected component %d, got %v", c2.ID, updated.AffectedComponentIDs)
	}
}

func TestMaintenancePatchInProgressAffectedRejected(t *testing.T) {
	srv, q, _, _ := newMaintTestServer(t)
	client := adminClient(t, srv, q)
	w := createWindow(t, client, srv, q, -10*time.Minute, time.Hour)
	c2 := seedComponent(t, q, "alt")
	resp := doJSON(t, client, srv, http.MethodPatch, "/api/maintenance/"+strconv.FormatInt(w.ID, 10), map[string]any{
		"affected_component_ids": []int64{c2.ID},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}
