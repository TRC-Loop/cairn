// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/TRC-Loop/cairn/internal/crypto"
	"github.com/TRC-Loop/cairn/internal/store"
)

const (
	defaultTickInterval  = 5 * time.Second
	defaultBatchSize     = 50
	deliveryTimeout      = 30 * time.Second
	gracePeriodOnCancel  = 30 * time.Second
)

// MaintenanceChecker is the narrow surface the dispatcher needs to suppress
// notifications during a maintenance window. Mirrors the contract that the
// incident package already defines.
type MaintenanceChecker interface {
	IsCheckUnderMaintenance(ctx context.Context, checkID int64) (bool, error)
}

type Dispatcher struct {
	db          *sql.DB
	q           *store.Queries
	senders     map[string]Sender
	logger      *slog.Logger
	interval    time.Duration
	maintenance MaintenanceChecker
	now         func() time.Time

	wg sync.WaitGroup
}

func NewDispatcher(db *sql.DB, q *store.Queries, secretBox *crypto.SecretBox, maintenance MaintenanceChecker, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		db:          db,
		q:           q,
		logger:      logger,
		interval:    defaultTickInterval,
		maintenance: maintenance,
		now:         time.Now,
		senders: map[string]Sender{
			ChannelEmail:   NewEmailSender(secretBox, logger),
			ChannelDiscord: NewDiscordSender(secretBox, logger),
			ChannelWebhook: NewWebhookSender(secretBox, logger),
		},
	}
}

func (d *Dispatcher) SetSender(typ string, s Sender) { d.senders[typ] = s }
func (d *Dispatcher) SetInterval(i time.Duration)    { d.interval = i }
func (d *Dispatcher) SetNow(fn func() time.Time)     { d.now = fn }

// EnqueueOptions controls Enqueue behaviour. AffectedCheckIDs is used for
// maintenance suppression: if all checks are under maintenance and the event
// is incident_opened/incident_updated, the delivery is silently dropped.
type EnqueueOptions struct {
	AffectedCheckIDs []int64
}

func (d *Dispatcher) Enqueue(ctx context.Context, channelID int64, eventType string, eventID int64, payload Payload, opts EnqueueOptions) (int64, error) {
	if d.shouldSuppress(ctx, eventType, opts.AffectedCheckIDs) {
		d.logger.Info("notification suppressed by maintenance",
			"channel_id", channelID, "event_type", eventType, "event_id", eventID)
		return 0, nil
	}

	channel, err := d.q.GetNotificationChannel(ctx, channelID)
	if err != nil {
		return 0, fmt.Errorf("get channel: %w", err)
	}
	if !channel.Enabled {
		d.logger.Info("notification skipped: channel disabled",
			"channel_id", channelID, "event_type", eventType)
		return 0, nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	row, err := d.q.CreateNotificationDelivery(ctx, store.CreateNotificationDeliveryParams{
		ChannelID:     channelID,
		EventType:     eventType,
		EventID:       eventID,
		PayloadJson:   string(body),
		NextAttemptAt: sql.NullTime{Time: d.now().UTC(), Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("create delivery: %w", err)
	}
	d.logger.Debug("delivery enqueued", "id", row.ID, "channel_id", channelID, "event_type", eventType)
	return row.ID, nil
}

func (d *Dispatcher) shouldSuppress(ctx context.Context, eventType string, checkIDs []int64) bool {
	if d.maintenance == nil || len(checkIDs) == 0 {
		return false
	}
	if eventType != EventIncidentOpened && eventType != EventIncidentUpdated {
		return false
	}
	for _, id := range checkIDs {
		under, err := d.maintenance.IsCheckUnderMaintenance(ctx, id)
		if err != nil {
			d.logger.Warn("maintenance check failed; not suppressing", "check_id", id, "err", err)
			return false
		}
		if !under {
			return false
		}
	}
	return true
}

func (d *Dispatcher) Start(ctx context.Context) error {
	d.logger.Info("notification dispatcher started", "interval", d.interval)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("notification dispatcher stopping; waiting for in-flight deliveries")
			drained := make(chan struct{})
			go func() {
				d.wg.Wait()
				close(drained)
			}()
			select {
			case <-drained:
			case <-time.After(gracePeriodOnCancel):
				d.logger.Warn("dispatcher drain grace period exceeded")
			}
			return nil
		case <-ticker.C:
			d.tick(context.Background())
		}
	}
}

func (d *Dispatcher) tick(ctx context.Context) {
	now := d.now().UTC()
	rows, err := d.q.ListPendingDeliveries(ctx, store.ListPendingDeliveriesParams{
		NextAttemptAt: sql.NullTime{Time: now, Valid: true},
		Limit:         defaultBatchSize,
	})
	if err != nil {
		d.logger.Error("list pending deliveries failed", "err", err)
		return
	}
	for _, row := range rows {
		row := row
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.process(row)
		}()
	}
}

// RunOnce processes one tick synchronously. Exported for tests.
func (d *Dispatcher) RunOnce(ctx context.Context) {
	d.tick(ctx)
	d.wg.Wait()
}

func (d *Dispatcher) process(row store.NotificationDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), deliveryTimeout)
	defer cancel()

	channel, err := d.q.GetNotificationChannel(ctx, row.ChannelID)
	if err != nil {
		d.logger.Error("get channel for delivery failed", "delivery_id", row.ID, "err", err)
		return
	}
	if !channel.Enabled {
		// Channel was turned off after enqueue: mark failed and move on.
		_ = d.q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{
			LastError: sql.NullString{String: "channel disabled", Valid: true},
			ID:        row.ID,
		})
		return
	}

	sender, ok := d.senders[channel.Type]
	if !ok {
		_ = d.q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{
			LastError: sql.NullString{String: fmt.Sprintf("unknown channel type %q", channel.Type), Valid: true},
			ID:        row.ID,
		})
		return
	}

	if err := d.q.MarkDeliverySending(ctx, store.MarkDeliverySendingParams{
		LastAttemptedAt: sql.NullTime{Time: d.now().UTC(), Valid: true},
		ID:              row.ID,
	}); err != nil {
		d.logger.Error("mark sending failed", "delivery_id", row.ID, "err", err)
		return
	}

	var payload Payload
	if err := json.Unmarshal([]byte(row.PayloadJson), &payload); err != nil {
		_ = d.q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{
			LastError: sql.NullString{String: "invalid payload: " + err.Error(), Valid: true},
			ID:        row.ID,
		})
		return
	}

	sendErr := sender.Send(ctx, channel, payload)
	if sendErr == nil {
		if err := d.q.MarkDeliverySent(ctx, store.MarkDeliverySentParams{
			SentAt: sql.NullTime{Time: d.now().UTC(), Valid: true},
			ID:     row.ID,
		}); err != nil {
			d.logger.Error("mark sent failed", "delivery_id", row.ID, "err", err)
			return
		}
		d.logger.Info("delivery sent",
			"delivery_id", row.ID, "channel_id", channel.ID, "channel_name", channel.Name,
			"event_type", row.EventType, "event_id", row.EventID, "attempt", row.AttemptCount+1)
		return
	}

	nextAttempt := row.AttemptCount + 1
	if nextAttempt > channel.RetryMax {
		_ = d.q.MarkDeliveryFailed(ctx, store.MarkDeliveryFailedParams{
			LastError: sql.NullString{String: truncateErr(sendErr.Error()), Valid: true},
			ID:        row.ID,
		})
		d.logger.Warn("delivery permanently failed",
			"delivery_id", row.ID, "channel_id", channel.ID, "channel_name", channel.Name,
			"event_type", row.EventType, "attempts", nextAttempt, "err", sendErr)
		return
	}

	backoff := backoffDuration(channel.RetryBackoffSeconds, nextAttempt)
	next := d.now().UTC().Add(backoff)
	if err := d.q.MarkDeliveryRetry(ctx, store.MarkDeliveryRetryParams{
		LastError:     sql.NullString{String: truncateErr(sendErr.Error()), Valid: true},
		NextAttemptAt: sql.NullTime{Time: next, Valid: true},
		ID:            row.ID,
	}); err != nil {
		d.logger.Error("mark retry failed", "delivery_id", row.ID, "err", err)
		return
	}
	d.logger.Info("delivery scheduled for retry",
		"delivery_id", row.ID, "channel_id", channel.ID, "channel_name", channel.Name,
		"attempt", nextAttempt, "next_at", next, "err", sendErr)
}

// backoffDuration returns initial * 2^(attempt-1). attempt is 1-indexed (1st
// retry uses initial; 2nd uses 2x; 3rd uses 4x; ...).
func backoffDuration(initialSeconds, attempt int64) time.Duration {
	if initialSeconds < 1 {
		initialSeconds = 1
	}
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 30 {
		attempt = 30
	}
	mult := math.Pow(2, float64(attempt-1))
	return time.Duration(float64(initialSeconds)*mult) * time.Second
}

func truncateErr(s string) string {
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// ErrChannelDisabled is returned when an enqueue would target a disabled channel.
var ErrChannelDisabled = errors.New("notification channel disabled")

// NotifyChecks resolves all distinct channels associated with the given check
// IDs and enqueues a delivery on each. Returns the number of deliveries
// enqueued (suppressions and disabled channels do not count).
func (d *Dispatcher) NotifyChecks(ctx context.Context, eventType string, eventID int64, payload Payload, checkIDs []int64) (int, error) {
	if len(checkIDs) == 0 {
		return 0, nil
	}
	seen := map[int64]struct{}{}
	var channelIDs []int64
	for _, cid := range checkIDs {
		ids, err := d.q.ListChannelsForCheck(ctx, cid)
		if err != nil {
			return 0, fmt.Errorf("list channels for check %d: %w", cid, err)
		}
		for _, id := range ids {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			channelIDs = append(channelIDs, id)
		}
	}
	count := 0
	for _, id := range channelIDs {
		deliveryID, err := d.Enqueue(ctx, id, eventType, eventID, payload, EnqueueOptions{AffectedCheckIDs: checkIDs})
		if err != nil {
			d.logger.Error("enqueue failed", "channel_id", id, "event_type", eventType, "err", err)
			continue
		}
		if deliveryID > 0 {
			count++
		}
	}
	return count, nil
}
