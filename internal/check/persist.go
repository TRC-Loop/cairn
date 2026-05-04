// SPDX-License-Identifier: AGPL-3.0-or-later
package check

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/store"
)

// IncidentService is the minimal surface the check package needs from the
// incident service. Kept as an interface so tests can pass nil or a fake.
type IncidentService interface {
	CreateAutoFromCheckFailure(ctx context.Context, c store.Check) (int64, error)
	FindOpenForCheck(ctx context.Context, checkID int64) (store.Incident, bool, error)
	Transition(ctx context.Context, incidentID int64, to incident.Status, userID *int64, message string) error
}

func PersistResult(ctx context.Context, db *sql.DB, q *store.Queries, incidentSvc IncidentService, c store.Check, result Result) error {
	var metadataJSON sql.NullString
	if result.Metadata != nil {
		b, err := json.Marshal(result.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		metadataJSON = sql.NullString{String: string(b), Valid: true}
	}

	var latency sql.NullInt64
	if result.LatencyMs != nil {
		latency = sql.NullInt64{Int64: int64(*result.LatencyMs), Valid: true}
	}

	var errMsg sql.NullString
	if result.ErrorMessage != "" {
		errMsg = sql.NullString{String: result.ErrorMessage, Valid: true}
	}

	now := time.Now().UTC()

	var successes, failures int64
	switch result.Status {
	case StatusUp, StatusDegraded:
		successes = c.ConsecutiveSuccesses + 1
		failures = 0
	case StatusDown:
		failures = c.ConsecutiveFailures + 1
		successes = 0
	case StatusUnknown:
		successes = 0
		failures = 0
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := q.WithTx(tx)

	if err := qtx.InsertCheckResult(ctx, store.InsertCheckResultParams{
		CheckID:              c.ID,
		CheckedAt:            now,
		Status:               string(result.Status),
		LatencyMs:            latency,
		ErrorMessage:         errMsg,
		ResponseMetadataJson: metadataJSON,
	}); err != nil {
		return fmt.Errorf("insert check result: %w", err)
	}

	if err := qtx.UpdateCheckStatus(ctx, store.UpdateCheckStatusParams{
		LastStatus:           string(result.Status),
		LastLatencyMs:        latency,
		LastCheckedAt:        sql.NullTime{Time: now, Valid: true},
		ConsecutiveFailures:  failures,
		ConsecutiveSuccesses: successes,
		ID:                   c.ID,
	}); err != nil {
		return fmt.Errorf("update check status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	if incidentSvc == nil {
		return nil
	}

	updated := c
	updated.LastStatus = string(result.Status)
	updated.ConsecutiveFailures = failures
	updated.ConsecutiveSuccesses = successes

	switch result.Status {
	case StatusDown:
		if failures >= c.FailureThreshold && c.FailureThreshold > 0 {
			if _, err := incidentSvc.CreateAutoFromCheckFailure(ctx, updated); err != nil {
				return fmt.Errorf("auto-incident create: %w", err)
			}
		}
	case StatusUp, StatusDegraded:
		if successes >= c.RecoveryThreshold && c.RecoveryThreshold > 0 {
			open, ok, err := incidentSvc.FindOpenForCheck(ctx, c.ID)
			if err != nil {
				return fmt.Errorf("find open incident: %w", err)
			}
			if ok {
				msg := fmt.Sprintf("Cairn detected %s has recovered.", c.Name)
				if err := incidentSvc.Transition(ctx, open.ID, incident.StatusResolved, nil, msg); err != nil {
					return fmt.Errorf("auto-resolve incident: %w", err)
				}
			}
		}
	}
	return nil
}
