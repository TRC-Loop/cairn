// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"

	"github.com/TRC-Loop/cairn/internal/incident"
	"github.com/TRC-Loop/cairn/internal/maintenance"
)

// IncidentAdapter wraps a Dispatcher to satisfy incident.Notifier without
// forcing the incident package to import notifier.
type IncidentAdapter struct {
	d *Dispatcher
}

func NewIncidentAdapter(d *Dispatcher) *IncidentAdapter {
	return &IncidentAdapter{d: d}
}

func (a *IncidentAdapter) NotifyChecks(ctx context.Context, eventType string, eventID int64, p incident.NotifierPayload, checkIDs []int64) (int, error) {
	links := make([]Link, 0, len(p.Links))
	for _, l := range p.Links {
		links = append(links, Link{Label: l.Label, URL: l.URL})
	}
	payload := Payload{
		EventType: p.EventType,
		EventID:   p.EventID,
		Subject:   p.Subject,
		Body:      p.Body,
		Severity:  p.Severity,
		Links:     links,
		Timestamp: p.Timestamp,
	}
	return a.d.NotifyChecks(ctx, eventType, eventID, payload, checkIDs)
}

// MaintenanceAdapter satisfies maintenance.MaintenanceNotifier.
type MaintenanceAdapter struct {
	d *Dispatcher
}

func NewMaintenanceAdapter(d *Dispatcher) *MaintenanceAdapter {
	return &MaintenanceAdapter{d: d}
}

func (a *MaintenanceAdapter) NotifyChecks(ctx context.Context, eventType string, eventID int64, p maintenance.MaintenancePayload, checkIDs []int64) (int, error) {
	payload := Payload{
		EventType: p.EventType,
		EventID:   p.EventID,
		Subject:   p.Subject,
		Body:      p.Body,
		Severity:  p.Severity,
		Timestamp: p.Timestamp,
	}
	return a.d.NotifyChecks(ctx, eventType, eventID, payload, checkIDs)
}
