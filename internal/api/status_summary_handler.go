// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type statusSummaryResponse struct {
	OverallStatus    string `json:"overall_status"`
	ActiveIncidents  int    `json:"active_incidents"`
	InMaintenance    bool   `json:"in_maintenance"`
}

type statusSummaryCache struct {
	mu       sync.Mutex
	resp     statusSummaryResponse
	expires  time.Time
}

const summaryCacheTTL = 30 * time.Second

func statusSummaryHandler(q *store.Queries, logger *slog.Logger) http.HandlerFunc {
	cache := &statusSummaryCache{}
	return func(w http.ResponseWriter, r *http.Request) {
		cache.mu.Lock()
		if time.Now().Before(cache.expires) {
			resp := cache.resp
			cache.mu.Unlock()
			writeJSON(w, http.StatusOK, resp)
			return
		}
		cache.mu.Unlock()

		ctx := r.Context()
		checks, err := q.ListChecks(ctx)
		if err != nil {
			logger.Error("status summary: list checks failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		incidents, err := q.ListActiveIncidents(ctx)
		if err != nil {
			logger.Error("status summary: list active incidents failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}
		maint, err := q.ListActiveMaintenance(ctx)
		if err != nil {
			logger.Error("status summary: list active maintenance failed", "err", err)
			writeError(w, http.StatusInternalServerError, CodeInternalError, "internal error", nil)
			return
		}

		var down, degraded, total int
		for _, c := range checks {
			if !c.Enabled {
				continue
			}
			total++
			switch c.LastStatus {
			case "down":
				down++
			case "degraded":
				degraded++
			}
		}

		inMaint := false
		now := time.Now().UTC()
		for _, m := range maint {
			if m.State == "in_progress" || (!m.StartsAt.After(now) && m.EndsAt.After(now)) {
				inMaint = true
				break
			}
		}

		overall := "operational"
		switch {
		case total == 0 && len(incidents) == 0:
			overall = "operational"
		case down > 0 && total > 0 && down*2 > total:
			overall = "major_outage"
		case down > 0:
			overall = "partial_outage"
		case degraded > 0:
			overall = "degraded"
		case len(incidents) > 0:
			overall = "partial_outage"
		}

		resp := statusSummaryResponse{
			OverallStatus:   overall,
			ActiveIncidents: len(incidents),
			InMaintenance:   inMaint,
		}

		cache.mu.Lock()
		cache.resp = resp
		cache.expires = time.Now().Add(summaryCacheTTL)
		cache.mu.Unlock()

		writeJSON(w, http.StatusOK, resp)
	}
}
