// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type DashboardHandler struct {
	q      *store.Queries
	logger *slog.Logger

	mu     sync.Mutex
	cached dashboardResponse
	cacheT time.Time
}

func NewDashboardHandler(q *store.Queries, logger *slog.Logger) *DashboardHandler {
	return &DashboardHandler{q: q, logger: logger}
}

type dashboardMonitors struct {
	Total    int64 `json:"total"`
	Up       int64 `json:"up"`
	Down     int64 `json:"down"`
	Degraded int64 `json:"degraded"`
	Unknown  int64 `json:"unknown"`
	Paused   int64 `json:"paused"`
}

type dashboardActiveIncidents struct {
	Count       int64  `json:"count"`
	LatestTitle string `json:"latest_title,omitempty"`
	LatestID    int64  `json:"latest_id,omitempty"`
}

type dashboardUptime struct {
	Percentage          float64 `json:"percentage"`
	Previous24hPct      float64 `json:"previous_24h_percentage"`
	HasCurrent          bool    `json:"has_current"`
	HasPrevious         bool    `json:"has_previous"`
}

type dashboardMaintenance struct {
	InProgressCount int64                   `json:"in_progress_count"`
	NextWindow      *dashboardMaintenanceWin `json:"next_window,omitempty"`
}

type dashboardMaintenanceWin struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
}

type dashboardActivity struct {
	Type      string    `json:"type"`
	ID        int64     `json:"id"`
	Label     string    `json:"label"`
	Timestamp time.Time `json:"timestamp"`
}

type dashboardResponse struct {
	Monitors        dashboardMonitors        `json:"monitors"`
	ActiveIncidents dashboardActiveIncidents `json:"active_incidents"`
	Uptime24h       dashboardUptime          `json:"uptime_24h"`
	Maintenance     dashboardMaintenance     `json:"maintenance"`
	RecentActivity  []dashboardActivity      `json:"recent_activity"`
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	if time.Since(h.cacheT) < 60*time.Second && !h.cacheT.IsZero() {
		out := h.cached
		h.mu.Unlock()
		writeJSON(w, http.StatusOK, out)
		return
	}
	h.mu.Unlock()

	out, err := h.build(r.Context())
	if err != nil {
		h.logger.Error("dashboard build failed", "err", err)
		writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
		return
	}
	h.mu.Lock()
	h.cached = out
	h.cacheT = time.Now()
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (h *DashboardHandler) build(ctx context.Context) (dashboardResponse, error) {
	var resp dashboardResponse

	counts, err := h.q.CountChecksByStatus(ctx)
	if err != nil {
		return resp, err
	}
	for _, row := range counts {
		resp.Monitors.Total += row.Count
		switch row.Status {
		case "up":
			resp.Monitors.Up = row.Count
		case "down":
			resp.Monitors.Down = row.Count
		case "degraded":
			resp.Monitors.Degraded = row.Count
		case "paused":
			resp.Monitors.Paused = row.Count
		default:
			resp.Monitors.Unknown += row.Count
		}
	}

	active, err := h.q.ListActiveIncidents(ctx)
	if err != nil {
		return resp, err
	}
	resp.ActiveIncidents.Count = int64(len(active))
	if len(active) > 0 {
		latest := active[0]
		for _, inc := range active[1:] {
			if inc.StartedAt.After(latest.StartedAt) {
				latest = inc
			}
		}
		resp.ActiveIncidents.LatestID = latest.ID
		resp.ActiveIncidents.LatestTitle = latest.Title
	}

	now := time.Now().UTC()
	cur, err := h.q.DashboardUptime24h(ctx, now.Add(-24*time.Hour))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return resp, err
	}
	prev, err := h.q.DashboardUptime24h(ctx, now.Add(-48*time.Hour))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return resp, err
	}
	if cur.TotalCount > 0 {
		resp.Uptime24h.HasCurrent = true
		resp.Uptime24h.Percentage = round2(float64(cur.UpCount) / float64(cur.TotalCount) * 100)
	}
	prevOnlyTotal := prev.TotalCount - cur.TotalCount
	prevOnlyUp := prev.UpCount - cur.UpCount
	if prevOnlyTotal > 0 {
		resp.Uptime24h.HasPrevious = true
		resp.Uptime24h.Previous24hPct = round2(float64(prevOnlyUp) / float64(prevOnlyTotal) * 100)
	}

	activeM, err := h.q.ListActiveMaintenance(ctx)
	if err != nil {
		return resp, err
	}
	resp.Maintenance.InProgressCount = int64(len(activeM))
	upcoming, err := h.q.ListUpcomingMaintenance(ctx)
	if err != nil {
		return resp, err
	}
	if len(upcoming) > 0 {
		next := upcoming[0]
		for _, mw := range upcoming[1:] {
			if mw.StartsAt.Before(next.StartsAt) {
				next = mw
			}
		}
		resp.Maintenance.NextWindow = &dashboardMaintenanceWin{
			ID: next.ID, Title: next.Title, StartsAt: next.StartsAt,
		}
	}

	since := now.Add(-7 * 24 * time.Hour)
	incidents, err := h.q.ListIncidentsSince(ctx, since)
	if err == nil {
		for _, inc := range incidents {
			resp.RecentActivity = append(resp.RecentActivity, dashboardActivity{
				Type: "incident_opened", ID: inc.ID, Label: inc.Title, Timestamp: inc.StartedAt,
			})
			if inc.ResolvedAt.Valid && inc.ResolvedAt.Time.After(since) {
				resp.RecentActivity = append(resp.RecentActivity, dashboardActivity{
					Type: "incident_resolved", ID: inc.ID, Label: inc.Title, Timestamp: inc.ResolvedAt.Time,
				})
			}
		}
	}
	for _, mw := range activeM {
		resp.RecentActivity = append(resp.RecentActivity, dashboardActivity{
			Type: "maintenance_started", ID: mw.ID, Label: mw.Title, Timestamp: mw.StartsAt,
		})
	}
	// newest first, cap at 10
	for i := 0; i < len(resp.RecentActivity); i++ {
		for j := i + 1; j < len(resp.RecentActivity); j++ {
			if resp.RecentActivity[j].Timestamp.After(resp.RecentActivity[i].Timestamp) {
				resp.RecentActivity[i], resp.RecentActivity[j] = resp.RecentActivity[j], resp.RecentActivity[i]
			}
		}
	}
	if len(resp.RecentActivity) > 10 {
		resp.RecentActivity = resp.RecentActivity[:10]
	}
	if resp.RecentActivity == nil {
		resp.RecentActivity = []dashboardActivity{}
	}

	return resp, nil
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
