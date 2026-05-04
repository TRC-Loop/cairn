// SPDX-License-Identifier: AGPL-3.0-or-later
package incident

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

type stubMaintenance struct {
	underMaintenance bool
	calls            int
}

func (s *stubMaintenance) IsCheckUnderMaintenance(ctx context.Context, checkID int64) (bool, error) {
	s.calls++
	return s.underMaintenance, nil
}

func TestAutoIncidentSuppressedByMaintenance(t *testing.T) {
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &stubMaintenance{underMaintenance: true}
	svc := NewService(db, q, logger, stub)
	ctx := context.Background()

	c := createCheck(t, q, "api")

	id, err := svc.CreateAutoFromCheckFailure(ctx, c)
	if err != nil {
		t.Fatalf("auto: %v", err)
	}
	if id != 0 {
		t.Fatalf("expected incident id 0 under maintenance, got %d", id)
	}
	if stub.calls != 1 {
		t.Fatalf("expected maintenance consulted once, got %d", stub.calls)
	}

	active, err := q.ListActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no incidents, got %d", len(active))
	}

	stub.underMaintenance = false
	id, err = svc.CreateAutoFromCheckFailure(ctx, c)
	if err != nil {
		t.Fatalf("auto after maintenance: %v", err)
	}
	if id == 0 {
		t.Fatal("expected incident created once maintenance ends")
	}
	active2, err := q.ListActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(active2) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(active2))
	}
}

// Regression: a maintenance window that begins after an incident is already
// open must not cause CreateAutoFromCheckFailure to return 0 — the caller
// would lose the incident ID and stop tracking it. FindOpenForCheck runs
// before the maintenance gate to keep existing incidents returned verbatim.
func TestAutoIncidentPersistsWhenMaintenanceStartsMidIncident(t *testing.T) {
	db, q := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &stubMaintenance{underMaintenance: false}
	svc := NewService(db, q, logger, stub)
	ctx := context.Background()

	c := createCheck(t, q, "api")

	id1, err := svc.CreateAutoFromCheckFailure(ctx, c)
	if err != nil {
		t.Fatalf("auto 1: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected incident created")
	}

	stub.underMaintenance = true
	id2, err := svc.CreateAutoFromCheckFailure(ctx, c)
	if err != nil {
		t.Fatalf("auto 2: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same incident returned across maintenance start, got %d vs %d", id2, id1)
	}

	active, err := q.ListActiveIncidents(ctx)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active incident, got %d", len(active))
	}
}
