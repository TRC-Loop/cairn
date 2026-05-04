// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

const (
	EventIncidentOpened     = "incident_opened"
	EventIncidentUpdated    = "incident_updated"
	EventIncidentResolved   = "incident_resolved"
	EventCheckRecovered     = "check_recovered"
	EventMaintenanceStarted = "maintenance_started"
	EventMaintenanceEnded   = "maintenance_ended"
	EventTest               = "test"
)

const (
	ChannelEmail   = "email"
	ChannelDiscord = "discord"
	ChannelWebhook = "webhook"
)

const (
	SeverityMinor    = "minor"
	SeverityMajor    = "major"
	SeverityCritical = "critical"
	SeverityInfo     = "info"
	SeverityMaint    = "maintenance"
)

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type Payload struct {
	EventType string    `json:"event_type"`
	EventID   int64     `json:"event_id"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Severity  string    `json:"severity"`
	Links     []Link    `json:"links"`
	Timestamp time.Time `json:"timestamp"`
}

type Sender interface {
	Send(ctx context.Context, channel store.NotificationChannel, payload Payload) error
}
