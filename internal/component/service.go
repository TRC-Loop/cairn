// SPDX-License-Identifier: AGPL-3.0-or-later
package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/TRC-Loop/cairn/internal/check"
	"github.com/TRC-Loop/cairn/internal/store"
)

type Service struct {
	db     *sql.DB
	q      *store.Queries
	logger *slog.Logger
}

func NewService(db *sql.DB, q *store.Queries, logger *slog.Logger) *Service {
	return &Service{db: db, q: q, logger: logger}
}

type CreateInput struct {
	Name         string
	Description  string
	DisplayOrder int64
}

type UpdateInput struct {
	Name         string
	Description  string
	DisplayOrder int64
}

func (s *Service) Get(ctx context.Context, id int64) (store.Component, error) {
	return s.q.GetComponent(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]store.Component, error) {
	return s.q.ListComponents(ctx)
}

func (s *Service) Create(ctx context.Context, in CreateInput) (store.Component, error) {
	if in.Name == "" {
		return store.Component{}, errors.New("component name required")
	}
	return s.q.CreateComponent(ctx, store.CreateComponentParams{
		Name:         in.Name,
		Description:  nullString(in.Description),
		DisplayOrder: in.DisplayOrder,
	})
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	if in.Name == "" {
		return errors.New("component name required")
	}
	return s.q.UpdateComponent(ctx, store.UpdateComponentParams{
		Name:         in.Name,
		Description:  nullString(in.Description),
		DisplayOrder: in.DisplayOrder,
		ID:           id,
	})
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.q.DeleteComponent(ctx, id)
}

// AggregateStatus derives a component-level status from its member checks.
func (s *Service) AggregateStatus(ctx context.Context, componentID int64) (check.Status, error) {
	checks, err := s.q.ListChecksForComponent(ctx, sql.NullInt64{Int64: componentID, Valid: true})
	if err != nil {
		return check.StatusUnknown, fmt.Errorf("list checks for component: %w", err)
	}
	if len(checks) == 0 {
		return check.StatusUnknown, nil
	}
	var anyDegraded, anyKnown, allUp bool
	allUp = true
	for _, c := range checks {
		switch check.Status(c.LastStatus) {
		case check.StatusDown:
			return check.StatusDown, nil
		case check.StatusDegraded:
			anyDegraded = true
			anyKnown = true
			allUp = false
		case check.StatusUp:
			anyKnown = true
		default:
			allUp = false
		}
	}
	if anyDegraded {
		return check.StatusDegraded, nil
	}
	if anyKnown && allUp {
		return check.StatusUp, nil
	}
	return check.StatusUnknown, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
