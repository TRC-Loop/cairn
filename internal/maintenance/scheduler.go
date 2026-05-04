// SPDX-License-Identifier: AGPL-3.0-or-later
package maintenance

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

const tickInterval = 30 * time.Second

// MaintenanceNotifier dispatches maintenance lifecycle notifications. The
// scheduler resolves channels via the affected components → checks → channels
// path before calling here.
type MaintenanceNotifier interface {
	NotifyChecks(ctx context.Context, eventType string, eventID int64, payload MaintenancePayload, checkIDs []int64) (int, error)
}

type MaintenancePayload struct {
	EventType string
	EventID   int64
	Subject   string
	Body      string
	Severity  string
	Timestamp time.Time
}

const (
	EventMaintenanceStarted = "maintenance_started"
	EventMaintenanceEnded   = "maintenance_ended"
)

type StateScheduler struct {
	q        *store.Queries
	logger   *slog.Logger
	notifier MaintenanceNotifier
}

func NewStateScheduler(q *store.Queries, logger *slog.Logger) *StateScheduler {
	return &StateScheduler{q: q, logger: logger}
}

func (s *StateScheduler) SetNotifier(n MaintenanceNotifier) { s.notifier = n }

func (s *StateScheduler) Start(ctx context.Context) error {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	s.logger.Info("maintenance state scheduler started", "tick", tickInterval)

	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("maintenance state scheduler stopped")
			return nil
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

// tick advances maintenance windows through their auto state transitions and
// emits notifications for each transition. We iterate per-window (rather than
// the bulk SQL update) so we can fire one event per window with the right
// affected-checks list.
func (s *StateScheduler) tick(ctx context.Context) {
	now := time.Now().UTC()

	// Auto-start: scheduled → in_progress where starts_at <= now.
	upcoming, err := s.q.ListMaintenanceBetween(ctx, store.ListMaintenanceBetweenParams{
		StartsAt: now,
		EndsAt:   now,
	})
	if err != nil {
		s.logger.Error("list windows to start failed", "err", err)
	} else {
		for _, w := range upcoming {
			if w.State != StateScheduled {
				continue
			}
			if w.StartsAt.After(now) {
				continue
			}
			if err := s.q.UpdateMaintenanceState(ctx, store.UpdateMaintenanceStateParams{
				State: StateInProgress, ID: w.ID,
			}); err != nil {
				s.logger.Error("auto-start window failed", "id", w.ID, "err", err)
				continue
			}
			s.logger.Info("maintenance auto-started", "id", w.ID, "title", w.Title)
			s.notify(ctx, w, EventMaintenanceStarted)
		}
	}

	// Auto-complete: in_progress where ends_at <= now.
	active, err := s.q.ListActiveMaintenance(ctx)
	if err != nil {
		s.logger.Error("list active for completion failed", "err", err)
		return
	}
	for _, w := range active {
		if w.EndsAt.After(now) {
			continue
		}
		if err := s.q.UpdateMaintenanceState(ctx, store.UpdateMaintenanceStateParams{
			State: StateCompleted, ID: w.ID,
		}); err != nil {
			s.logger.Error("auto-complete window failed", "id", w.ID, "err", err)
			continue
		}
		s.logger.Info("maintenance auto-completed", "id", w.ID, "title", w.Title)
		s.notify(ctx, w, EventMaintenanceEnded)
	}
}

// RunOnce runs a single tick; exported for tests.
func (s *StateScheduler) RunOnce(ctx context.Context) {
	s.tick(ctx)
}

// notify resolves the affected-component → check → channel chain for the
// window and dispatches a maintenance lifecycle notification. Errors are
// logged and swallowed.
func (s *StateScheduler) notify(ctx context.Context, w store.MaintenanceWindow, eventType string) {
	if s.notifier == nil {
		return
	}
	componentIDs, err := s.q.ListComponentsForMaintenance(ctx, w.ID)
	if err != nil {
		s.logger.Error("list components for maintenance failed", "id", w.ID, "err", err)
		return
	}
	seen := map[int64]struct{}{}
	var checkIDs []int64
	for _, cid := range componentIDs {
		checks, err := s.q.ListChecksForComponent(ctx, sql.NullInt64{Int64: cid, Valid: true})
		if err != nil {
			s.logger.Error("list checks for component failed", "component_id", cid, "err", err)
			continue
		}
		for _, c := range checks {
			if _, ok := seen[c.ID]; ok {
				continue
			}
			seen[c.ID] = struct{}{}
			checkIDs = append(checkIDs, c.ID)
		}
	}
	if len(checkIDs) == 0 {
		return
	}
	subject := fmt.Sprintf("Maintenance started: %s", w.Title)
	body := descOrDefault(w, "Scheduled maintenance is now in progress.")
	if eventType == EventMaintenanceEnded {
		subject = fmt.Sprintf("Maintenance ended: %s", w.Title)
		body = descOrDefault(w, "Scheduled maintenance is now complete.")
	}
	if _, err := s.notifier.NotifyChecks(ctx, eventType, w.ID, MaintenancePayload{
		EventType: eventType,
		EventID:   w.ID,
		Subject:   subject,
		Body:      body,
		Severity:  "maintenance",
		Timestamp: time.Now().UTC(),
	}, checkIDs); err != nil {
		s.logger.Error("maintenance notify failed", "id", w.ID, "event", eventType, "err", err)
	}
}

func descOrDefault(w store.MaintenanceWindow, fallback string) string {
	if w.Description.Valid && w.Description.String != "" {
		return w.Description.String
	}
	return fallback
}
