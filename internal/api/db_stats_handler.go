// SPDX-License-Identifier: AGPL-3.0-or-later
package api

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"github.com/TRC-Loop/cairn/internal/store"
)

type DBStatsHandler struct {
	db     *sql.DB
	q      *store.Queries
	path   string
	logger *slog.Logger
}

func NewDBStatsHandler(db *sql.DB, q *store.Queries, path string, logger *slog.Logger) *DBStatsHandler {
	return &DBStatsHandler{db: db, q: q, path: path, logger: logger}
}

type dbStatsResponse struct {
	Path     string         `json:"path"`
	SizeBytes int64         `json:"size_bytes"`
	Rows     map[string]int64 `json:"rows"`
	Health   string         `json:"health"`
}

func (h *DBStatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	var size int64
	if fi, err := os.Stat(h.path); err == nil {
		size = fi.Size()
	}

	rows := map[string]int64{}
	tables := []string{
		"checks", "check_results", "check_results_hourly", "check_results_daily",
		"incidents", "users", "components", "status_pages", "maintenance_windows",
	}
	for _, t := range tables {
		var n int64
		if err := h.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+t).Scan(&n); err != nil {
			h.logger.Warn("count failed", "table", t, "err", err)
			continue
		}
		rows[t] = n
	}

	health := classifyDBHealth(size)
	writeJSON(w, http.StatusOK, dbStatsResponse{
		Path:      h.path,
		SizeBytes: size,
		Rows:      rows,
		Health:    health,
	})
}

func classifyDBHealth(size int64) string {
	const mb = 1024 * 1024
	const gb = 1024 * mb
	switch {
	case size < 500*mb:
		return "good"
	case size < 2*gb:
		return "caution"
	default:
		return "warning"
	}
}
