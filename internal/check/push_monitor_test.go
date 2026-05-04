// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

func TestPushMonitorMarksMissedHeartbeat(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "hb",
		Type:              "push",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		Retries:           0,
		FailureThreshold:  1,
		RecoveryThreshold: 1,
		ConfigJson:        `{"grace_period_seconds":60}`,
	})
	if err != nil {
		t.Fatalf("create check: %v", err)
	}

	threeMinAgo := time.Now().UTC().Add(-3 * time.Minute)
	if err := q.UpdateCheckStatus(ctx, store.UpdateCheckStatusParams{
		LastStatus:    "up",
		LastCheckedAt: sql.NullTime{Time: threeMinAgo, Valid: true},
		ID:            c.ID,
	}); err != nil {
		t.Fatalf("update status: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewPushMonitor(db, q, logger, nil)
	if err := m.RunOnce(ctx); err != nil {
		t.Fatalf("run once: %v", err)
	}

	rows, err := q.GetRecentResults(ctx, store.GetRecentResultsParams{CheckID: c.ID, Limit: 10})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 result, got %d", len(rows))
	}
	if rows[0].Status != string(StatusDown) {
		t.Fatalf("expected down, got %s", rows[0].Status)
	}
	if !rows[0].ErrorMessage.Valid || rows[0].ErrorMessage.String != "missed heartbeat" {
		t.Fatalf("expected missed heartbeat error, got %#v", rows[0].ErrorMessage)
	}

	updated, err := q.GetCheck(ctx, c.ID)
	if err != nil {
		t.Fatalf("get check: %v", err)
	}
	if updated.LastStatus != string(StatusDown) {
		t.Fatalf("expected check status down, got %s", updated.LastStatus)
	}
}

func TestPushMonitorDedupsWhileDown(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "hb",
		Type:              "push",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		FailureThreshold:  1,
		RecoveryThreshold: 1,
		ConfigJson:        `{"grace_period_seconds":60}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	threeMinAgo := time.Now().UTC().Add(-3 * time.Minute)
	if err := q.UpdateCheckStatus(ctx, store.UpdateCheckStatusParams{
		LastStatus:    "up",
		LastCheckedAt: sql.NullTime{Time: threeMinAgo, Valid: true},
		ID:            c.ID,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewPushMonitor(db, q, logger, nil)

	if err := m.RunOnce(ctx); err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if err := m.RunOnce(ctx); err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if err := m.RunOnce(ctx); err != nil {
		t.Fatalf("run 3: %v", err)
	}

	rows, err := q.GetRecentResults(ctx, store.GetRecentResultsParams{CheckID: c.ID, Limit: 10})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 result after 3 ticks, got %d", len(rows))
	}
}

func TestPushMonitorSkipsRecentHeartbeat(t *testing.T) {
	db, q := openTestDB(t)
	ctx := context.Background()

	c, err := q.CreateCheck(ctx, store.CreateCheckParams{
		Name:              "hb",
		Type:              "push",
		Enabled:           true,
		IntervalSeconds:   60,
		TimeoutSeconds:    10,
		Retries:           0,
		FailureThreshold:  1,
		RecoveryThreshold: 1,
		ConfigJson:        `{"grace_period_seconds":60}`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	tenSecAgo := time.Now().UTC().Add(-10 * time.Second)
	if err := q.UpdateCheckStatus(ctx, store.UpdateCheckStatusParams{
		LastStatus:    "up",
		LastCheckedAt: sql.NullTime{Time: tenSecAgo, Valid: true},
		ID:            c.ID,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := NewPushMonitor(db, q, logger, nil)
	if err := m.RunOnce(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	rows, err := q.GetRecentResults(ctx, store.GetRecentResultsParams{CheckID: c.ID, Limit: 10})
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 results, got %d", len(rows))
	}
}
