// SPDX-License-Identifier: AGPL-3.0-or-later
package maintenance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

const (
	StateScheduled  = "scheduled"
	StateInProgress = "in_progress"
	StateCompleted  = "completed"
	StateCancelled  = "cancelled"
)

type Service struct {
	db       *sql.DB
	q        *store.Queries
	logger   *slog.Logger
	notifier MaintenanceNotifier
}

func NewService(db *sql.DB, q *store.Queries, logger *slog.Logger) *Service {
	return &Service{db: db, q: q, logger: logger}
}

func (s *Service) SetNotifier(n MaintenanceNotifier) { s.notifier = n }

var (
	ErrAlreadyEnded     = errors.New("maintenance window has ended and can't be edited")
	ErrInvalidStateOp   = errors.New("operation not valid for current state")
	ErrNotFound         = errors.New("maintenance window not found")
)

type CreateInput struct {
	Title              string
	Description        string
	StartsAt           time.Time
	EndsAt             time.Time
	CreatedByUserID    int64
	AffectedComponents []int64
}

type UpdateInput struct {
	Title              *string
	Description        *string
	StartsAt           *time.Time
	EndsAt             *time.Time
	AffectedComponents *[]int64
}

type ListFilter struct {
	Status   string
	Limit    int64
	Offset   int64
	Upcoming bool
	PastDays int
}

func (s *Service) Get(ctx context.Context, id int64) (store.MaintenanceWindow, error) {
	return s.q.GetMaintenanceWindow(ctx, id)
}

func (s *Service) List(ctx context.Context, limit int) ([]store.MaintenanceWindow, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.q.ListMaintenanceWindows(ctx, int64(limit))
}

func (s *Service) ListFiltered(ctx context.Context, f ListFilter) ([]store.MaintenanceWindow, int64, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.Upcoming {
		rows, err := s.q.ListUpcomingMaintenance(ctx)
		if err != nil {
			return nil, 0, err
		}
		return rows, int64(len(rows)), nil
	}
	if f.PastDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(f.PastDays) * 24 * time.Hour)
		rows, err := s.q.ListPastMaintenance(ctx, store.ListPastMaintenanceParams{
			EndsAt: cutoff, Limit: f.Limit,
		})
		if err != nil {
			return nil, 0, err
		}
		return rows, int64(len(rows)), nil
	}
	if f.Status != "" && f.Status != "all" {
		rows, err := s.q.ListMaintenanceFiltered(ctx, store.ListMaintenanceFilteredParams{
			State: f.Status, Limit: f.Limit, Offset: f.Offset,
		})
		if err != nil {
			return nil, 0, err
		}
		total, err := s.q.CountMaintenanceFiltered(ctx, f.Status)
		if err != nil {
			return nil, 0, err
		}
		return rows, total, nil
	}
	rows, err := s.q.ListMaintenanceAll(ctx, store.ListMaintenanceAllParams{
		Limit: f.Limit, Offset: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountMaintenanceAll(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) AffectedComponents(ctx context.Context, id int64) ([]store.Component, error) {
	return s.q.ListAffectedComponents(ctx, id)
}

func (s *Service) AffectedComponentIDs(ctx context.Context, id int64) ([]int64, error) {
	return s.q.ListComponentsForMaintenance(ctx, id)
}

func (s *Service) ListActive(ctx context.Context) ([]store.MaintenanceWindow, error) {
	return s.q.ListActiveMaintenance(ctx)
}

func (s *Service) ListUpcoming(ctx context.Context) ([]store.MaintenanceWindow, error) {
	return s.q.ListUpcomingMaintenance(ctx)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (store.MaintenanceWindow, error) {
	if in.Title == "" {
		return store.MaintenanceWindow{}, errors.New("title required")
	}
	if !in.EndsAt.After(in.StartsAt) {
		return store.MaintenanceWindow{}, errors.New("ends_at must be after starts_at")
	}
	now := time.Now().UTC()
	if !in.EndsAt.After(now) {
		return store.MaintenanceWindow{}, errors.New("ends_at must be in the future")
	}
	startInPast := !in.StartsAt.After(now)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return store.MaintenanceWindow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	state := StateScheduled
	if startInPast {
		state = StateInProgress
	}
	w, err := qtx.CreateMaintenanceWindow(ctx, store.CreateMaintenanceWindowParams{
		Title:           in.Title,
		Description:     nullString(in.Description),
		StartsAt:        in.StartsAt.UTC(),
		EndsAt:          in.EndsAt.UTC(),
		State:           state,
		CreatedByUserID: nullableUser(in.CreatedByUserID),
	})
	if err != nil {
		return store.MaintenanceWindow{}, fmt.Errorf("create: %w", err)
	}
	for _, cid := range in.AffectedComponents {
		if err := qtx.LinkComponent(ctx, store.LinkComponentParams{
			MaintenanceID: w.ID,
			ComponentID:   cid,
		}); err != nil {
			return store.MaintenanceWindow{}, fmt.Errorf("link component %d: %w", cid, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return store.MaintenanceWindow{}, fmt.Errorf("commit: %w", err)
	}
	if startInPast {
		s.logger.Info("maintenance created with starts_at in past, immediately in_progress",
			"id", w.ID, "starts_at", in.StartsAt)
	}
	return w, nil
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (store.MaintenanceWindow, error) {
	w, err := s.q.GetMaintenanceWindow(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.MaintenanceWindow{}, ErrNotFound
		}
		return store.MaintenanceWindow{}, err
	}
	if w.State == StateCompleted || w.State == StateCancelled {
		return store.MaintenanceWindow{}, ErrAlreadyEnded
	}

	title := w.Title
	if in.Title != nil {
		title = *in.Title
	}
	desc := ""
	if w.Description.Valid {
		desc = w.Description.String
	}
	if in.Description != nil {
		desc = *in.Description
	}
	startsAt := w.StartsAt
	endsAt := w.EndsAt
	if w.State == StateInProgress {
		// in_progress: only title, description, ends_at editable
		if in.StartsAt != nil && !in.StartsAt.Equal(w.StartsAt) {
			return store.MaintenanceWindow{}, errors.New("starts_at cannot be changed on an in-progress window")
		}
		if in.AffectedComponents != nil {
			return store.MaintenanceWindow{}, errors.New("affected_components cannot be changed on an in-progress window")
		}
		if in.EndsAt != nil {
			endsAt = *in.EndsAt
		}
	} else {
		if in.StartsAt != nil {
			startsAt = *in.StartsAt
		}
		if in.EndsAt != nil {
			endsAt = *in.EndsAt
		}
	}

	if title == "" {
		return store.MaintenanceWindow{}, errors.New("title required")
	}
	if !endsAt.After(startsAt) {
		return store.MaintenanceWindow{}, errors.New("ends_at must be after starts_at")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return store.MaintenanceWindow{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()
	qtx := s.q.WithTx(tx)

	if err := qtx.UpdateMaintenanceWindow(ctx, store.UpdateMaintenanceWindowParams{
		Title:       title,
		Description: nullString(desc),
		StartsAt:    startsAt.UTC(),
		EndsAt:      endsAt.UTC(),
		ID:          id,
	}); err != nil {
		return store.MaintenanceWindow{}, err
	}
	if in.AffectedComponents != nil && w.State == StateScheduled {
		existing, err := qtx.ListComponentsForMaintenance(ctx, id)
		if err != nil {
			return store.MaintenanceWindow{}, err
		}
		for _, cid := range existing {
			if err := qtx.UnlinkComponent(ctx, store.UnlinkComponentParams{
				MaintenanceID: id, ComponentID: cid,
			}); err != nil {
				return store.MaintenanceWindow{}, err
			}
		}
		for _, cid := range *in.AffectedComponents {
			if err := qtx.LinkComponent(ctx, store.LinkComponentParams{
				MaintenanceID: id, ComponentID: cid,
			}); err != nil {
				return store.MaintenanceWindow{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return store.MaintenanceWindow{}, err
	}
	updated, err := s.q.GetMaintenanceWindow(ctx, id)
	if err != nil {
		return store.MaintenanceWindow{}, err
	}
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, id int64) error {
	w, err := s.q.GetMaintenanceWindow(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if w.State == StateInProgress {
		return errors.New("use 'end now' instead to mark in-progress maintenance as complete early")
	}
	if w.State != StateScheduled {
		return ErrInvalidStateOp
	}
	rows, err := s.q.CancelMaintenance(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidStateOp
	}
	s.logger.Info("maintenance cancelled", "id", id)
	return nil
}

func (s *Service) EndNow(ctx context.Context, id int64) error {
	w, err := s.q.GetMaintenanceWindow(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if w.State != StateInProgress {
		return errors.New("only in_progress maintenance can be ended now")
	}
	rows, err := s.q.EndMaintenanceNow(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvalidStateOp
	}
	s.logger.Info("maintenance ended early", "id", id)
	updated, err := s.q.GetMaintenanceWindow(ctx, id)
	if err == nil {
		s.notifyEnded(ctx, updated)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	w, err := s.q.GetMaintenanceWindow(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if w.State == StateInProgress {
		return errors.New("cannot delete in_progress maintenance; use end-now first")
	}
	if w.State == StateCompleted {
		return errors.New("cannot delete completed maintenance; historical record preserved")
	}
	return s.q.DeleteMaintenanceWindow(ctx, id)
}

func (s *Service) notifyEnded(ctx context.Context, w store.MaintenanceWindow) {
	if s.notifier == nil {
		return
	}
	componentIDs, err := s.q.ListComponentsForMaintenance(ctx, w.ID)
	if err != nil {
		s.logger.Error("end-now notify: list components failed", "id", w.ID, "err", err)
		return
	}
	seen := map[int64]struct{}{}
	var checkIDs []int64
	for _, cid := range componentIDs {
		checks, err := s.q.ListChecksForComponent(ctx, sql.NullInt64{Int64: cid, Valid: true})
		if err != nil {
			s.logger.Error("end-now notify: list checks failed", "component_id", cid, "err", err)
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
	body := "Scheduled maintenance is now complete."
	if w.Description.Valid && w.Description.String != "" {
		body = w.Description.String
	}
	if _, err := s.notifier.NotifyChecks(ctx, EventMaintenanceEnded, w.ID, MaintenancePayload{
		EventType: EventMaintenanceEnded,
		EventID:   w.ID,
		Subject:   fmt.Sprintf("Maintenance ended: %s", w.Title),
		Body:      body,
		Severity:  "maintenance",
		Timestamp: time.Now().UTC(),
	}, checkIDs); err != nil {
		s.logger.Error("end-now notify failed", "id", w.ID, "err", err)
	}
}

func (s *Service) IsCheckUnderMaintenance(ctx context.Context, checkID int64) (bool, error) {
	c, err := s.q.GetCheck(ctx, checkID)
	if err != nil {
		return false, fmt.Errorf("get check: %w", err)
	}
	if !c.ComponentID.Valid {
		return false, nil
	}
	return s.IsComponentUnderMaintenance(ctx, c.ComponentID.Int64)
}

func (s *Service) IsComponentUnderMaintenance(ctx context.Context, componentID int64) (bool, error) {
	active, err := s.q.ListMaintenanceForComponent(ctx, componentID)
	if err != nil {
		return false, fmt.Errorf("list maintenance for component: %w", err)
	}
	return len(active) > 0, nil
}

func (s *Service) ActiveMaintenanceForComponent(ctx context.Context, componentID int64) ([]store.MaintenanceWindow, error) {
	return s.q.ListMaintenanceForComponent(ctx, componentID)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableUser(id int64) sql.NullInt64 {
	if id <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: id, Valid: true}
}
