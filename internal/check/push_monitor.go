// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

const (
	pushMonitorTick         = 30 * time.Second
	pushDownReassertInterval = 1 * time.Hour
)

type pushConfig struct {
	GracePeriodSeconds *int64 `json:"grace_period_seconds,omitempty"`
}

type PushMonitor struct {
	db          *sql.DB
	q           *store.Queries
	logger      *slog.Logger
	tickEvery   time.Duration
	incidentSvc IncidentService
}

func NewPushMonitor(db *sql.DB, q *store.Queries, logger *slog.Logger, incidentSvc IncidentService) *PushMonitor {
	return &PushMonitor{
		db:          db,
		q:           q,
		logger:      logger,
		tickEvery:   pushMonitorTick,
		incidentSvc: incidentSvc,
	}
}

func (m *PushMonitor) Start(ctx context.Context) error {
	ticker := time.NewTicker(m.tickEvery)
	defer ticker.Stop()
	m.logger.Info("push monitor started", "tick", m.tickEvery.String())

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("push monitor stopped")
			return nil
		case <-ticker.C:
			if err := m.RunOnce(ctx); err != nil {
				m.logger.Error("push monitor tick failed", "err", err)
			}
		}
	}
}

func (m *PushMonitor) RunOnce(ctx context.Context) error {
	checks, err := m.q.ListEnabledPushChecks(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, c := range checks {
		grace := 2 * c.IntervalSeconds
		var pc pushConfig
		if c.ConfigJson != "" {
			if err := json.Unmarshal([]byte(c.ConfigJson), &pc); err != nil {
				m.logger.Warn("push config parse failed", "id", c.ID, "err", err)
			} else if pc.GracePeriodSeconds != nil {
				grace = *pc.GracePeriodSeconds
			}
		}
		total := time.Duration(c.IntervalSeconds+grace) * time.Second

		var (
			lastSeenMeta any
			missedSince  int64
			lastSeen     time.Time
		)
		if !c.LastCheckedAt.Valid {
			if now.Sub(c.CreatedAt) < total {
				continue
			}
			lastSeenMeta = nil
			missedSince = int64(now.Sub(c.CreatedAt).Seconds())
		} else {
			lastSeen = c.LastCheckedAt.Time
			if now.Sub(lastSeen) <= total {
				continue
			}
			lastSeenMeta = lastSeen.UTC().Format(time.RFC3339Nano)
			missedSince = int64(now.Sub(lastSeen).Seconds())
		}

		if c.LastStatus == string(StatusDown) {
			recent, err := m.q.GetMostRecentResult(ctx, c.ID)
			if err == nil && now.Sub(recent.CheckedAt) < pushDownReassertInterval {
				continue
			}
			if err != nil && err != sql.ErrNoRows {
				m.logger.Warn("push monitor recent result lookup failed", "id", c.ID, "err", err)
			}
		}

		res := Result{
			Status:       StatusDown,
			ErrorMessage: "missed heartbeat",
			Metadata: map[string]any{
				"last_seen":            lastSeenMeta,
				"missed_since_seconds": missedSince,
			},
		}
		if err := PersistResult(ctx, m.db, m.q, m.incidentSvc, c, res); err != nil {
			m.logger.Error("push monitor persist failed", "id", c.ID, "err", err)
		}
	}
	return nil
}
