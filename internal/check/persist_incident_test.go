// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/store"
)

func TestPersistAutoCreatesAndResolvesIncident(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := incident.NewService(db, q, logger, nil)

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "api",
		Type:              "http",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  3,
		RecoveryThreshold: 1,
		ConfigJson:        `{}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Fail three times; first two shouldn't create an incident.
	for i := 0; i < 3; i++ {
		current, err := q.GetCheck(ctx, c.ID)
		if err != nil {
			t.Fatalf("get check: %v", err)
		}
		if err := PersistResult(ctx, db, q, svc, current, Result{
			Status:       StatusDown,
			ErrorMessage: "connection refused",
		}); err != nil {
			t.Fatalf("persist %d: %v", i, err)
		}
	}

	active, err := q.ListActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active incident, got %d", len(active))
	}
	incID := active[0].ID
	if !active[0].AutoCreated {
		t.Fatal("expected auto_created=true")
	}

	// One success crosses RecoveryThreshold=1 → resolve.
	current, err := q.GetCheck(ctx, c.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := PersistResult(ctx, db, q, svc, current, Result{Status: StatusUp}); err != nil {
		t.Fatalf("persist up: %v", err)
	}

	resolved, err := q.GetIncident(ctx, incID)
	if err != nil {
		t.Fatalf("get incident: %v", err)
	}
	if resolved.Status != string(incident.StatusResolved) {
		t.Fatalf("expected resolved, got %s", resolved.Status)
	}
	if !resolved.ResolvedAt.Valid {
		t.Fatal("expected resolved_at set")
	}

	updates, err := q.ListUpdatesForIncident(ctx, incID)
	if err != nil {
		t.Fatalf("list updates: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates (detected+recovered), got %d", len(updates))
	}
}

func TestPersistDownBelowThresholdNoIncident(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := incident.NewService(db, q, logger, nil)

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "api",
		Type:              "http",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  3,
		RecoveryThreshold: 1,
		ConfigJson:        `{}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	for i := 0; i < 2; i++ {
		current, _ := q.GetCheck(ctx, c.ID)
		if err := PersistResult(ctx, db, q, svc, current, Result{Status: StatusDown}); err != nil {
			t.Fatalf("persist: %v", err)
		}
	}
	active, err := q.ListActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected 0 incidents, got %d", len(active))
	}
}
