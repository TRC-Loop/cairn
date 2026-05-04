// SPDX-License-Identifier: AGPL-3.0-or-later
package notifier

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/TRC-Loop/cairn/internal/store"
)

type recordingSender struct {
	mu       sync.Mutex
	calls    int
	failWith error
}

func (r *recordingSender) Send(_ context.Context, _ store.NotificationChannel, _ Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.failWith
}

func (r *recordingSender) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type maintenanceStub struct {
	underMaintenance map[int64]bool
	err              error
}

func (m *maintenanceStub) IsCheckUnderMaintenance(_ context.Context, id int64) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.underMaintenance[id], nil
}

func newDispatcherForTest(t *testing.T, channelType string, retryMax, backoff int64, sender Sender) (*Dispatcher, store.NotificationChannel) {
	t.Helper()
	db, q := openTestDB(t)
	sb := testSecretBox(t)
	d := NewDispatcher(db, q, sb, nil, discardLogger())
	d.SetSender(channelType, sender)
	d.SetInterval(50 * time.Millisecond)
	now := time.Now().UTC()
	d.SetNow(func() time.Time { return now })

	channel, err := q.CreateNotificationChannel(context.Background(), store.CreateNotificationChannelParams{
		Name:                "test-" + channelType,
		Type:                channelType,
		Enabled:             true,
		ConfigJson:          `{}`,
		RetryMax:            retryMax,
		RetryBackoffSeconds: backoff,
	})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return d, channel
}

func TestDispatcherPendingToSent(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	ctx := context.Background()

	id, err := d.Enqueue(ctx, channel.ID, EventTest, 0, Payload{Subject: "hi"}, EnqueueOptions{})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id == 0 {
		t.Fatal("expected delivery id > 0")
	}

	d.RunOnce(ctx)

	if sender.count() != 1 {
		t.Fatalf("expected 1 send, got %d", sender.count())
	}
	got, err := d.q.GetNotificationDelivery(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "sent" {
		t.Fatalf("expected status=sent, got %s", got.Status)
	}
	if !got.SentAt.Valid {
		t.Fatal("expected sent_at to be set")
	}
}

func TestDispatcherRetryWithBackoff(t *testing.T) {
	sender := &recordingSender{failWith: errors.New("transient")}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 2, sender)
	ctx := context.Background()

	id, _ := d.Enqueue(ctx, channel.ID, EventTest, 0, Payload{Subject: "x"}, EnqueueOptions{})

	d.RunOnce(ctx)
	got, _ := d.q.GetNotificationDelivery(ctx, id)
	if got.Status != "pending" {
		t.Fatalf("after first attempt, expected pending, got %s", got.Status)
	}
	if got.AttemptCount != 1 {
		t.Fatalf("attempt_count=%d want 1", got.AttemptCount)
	}
	if !got.NextAttemptAt.Valid {
		t.Fatal("next_attempt_at should be set")
	}
	wantNext := d.now().Add(2 * time.Second)
	if !got.NextAttemptAt.Time.Equal(wantNext) {
		t.Fatalf("next_attempt: got %v want %v", got.NextAttemptAt.Time, wantNext)
	}

	if backoffDuration(2, 1) != 2*time.Second {
		t.Errorf("backoff(2,1)=%v", backoffDuration(2, 1))
	}
	if backoffDuration(2, 2) != 4*time.Second {
		t.Errorf("backoff(2,2)=%v", backoffDuration(2, 2))
	}
	if backoffDuration(2, 3) != 8*time.Second {
		t.Errorf("backoff(2,3)=%v", backoffDuration(2, 3))
	}
}

func TestDispatcherMaxRetriesMarksFailed(t *testing.T) {
	sender := &recordingSender{failWith: errors.New("persistent")}
	d, channel := newDispatcherForTest(t, ChannelEmail, 2, 1, sender)
	ctx := context.Background()
	id, _ := d.Enqueue(ctx, channel.ID, EventTest, 0, Payload{Subject: "x"}, EnqueueOptions{})

	for i := 0; i < 5; i++ {
		// Move clock forward so the row becomes due again.
		now := d.now().Add(1 * time.Hour)
		d.SetNow(func() time.Time { return now })
		d.RunOnce(ctx)
	}

	got, _ := d.q.GetNotificationDelivery(ctx, id)
	if got.Status != "failed" {
		t.Fatalf("expected failed, got %s (attempts=%d)", got.Status, got.AttemptCount)
	}
	if got.AttemptCount < 3 {
		t.Fatalf("expected >= 3 attempts (1 initial + 2 retries), got %d", got.AttemptCount)
	}
	if !got.LastError.Valid || got.LastError.String == "" {
		t.Fatal("expected last_error to be set")
	}
}

func TestDispatcherSuppressionByMaintenance(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	d.maintenance = &maintenanceStub{underMaintenance: map[int64]bool{42: true, 43: true}}
	ctx := context.Background()

	id, err := d.Enqueue(ctx, channel.ID, EventIncidentOpened, 7, Payload{Subject: "x"},
		EnqueueOptions{AffectedCheckIDs: []int64{42, 43}})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if id != 0 {
		t.Fatalf("expected suppressed enqueue (id=0), got %d", id)
	}

	d.RunOnce(ctx)
	if sender.count() != 0 {
		t.Fatalf("expected 0 sends after suppression, got %d", sender.count())
	}
}

func TestDispatcherSuppressionDoesNotApplyToResolved(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	d.maintenance = &maintenanceStub{underMaintenance: map[int64]bool{42: true}}
	ctx := context.Background()

	id, _ := d.Enqueue(ctx, channel.ID, EventIncidentResolved, 7, Payload{Subject: "x"},
		EnqueueOptions{AffectedCheckIDs: []int64{42}})
	if id == 0 {
		t.Fatal("resolved events must not be suppressed")
	}
}

func TestDispatcherSuppressionRequiresAllChecksUnderMaintenance(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	d.maintenance = &maintenanceStub{underMaintenance: map[int64]bool{42: true /* 43 not */}}
	ctx := context.Background()

	id, _ := d.Enqueue(ctx, channel.ID, EventIncidentOpened, 7, Payload{Subject: "x"},
		EnqueueOptions{AffectedCheckIDs: []int64{42, 43}})
	if id == 0 {
		t.Fatal("must not suppress when only some checks are under maintenance")
	}
}

func TestDispatcherDisabledChannelSkipsEnqueue(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	ctx := context.Background()

	if _, err := d.q.UpdateNotificationChannel(ctx, store.UpdateNotificationChannelParams{
		Name:                channel.Name,
		Enabled:             false,
		ConfigJson:          channel.ConfigJson,
		RetryMax:            channel.RetryMax,
		RetryBackoffSeconds: channel.RetryBackoffSeconds,
		ID:                  channel.ID,
	}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	id, _ := d.Enqueue(ctx, channel.ID, EventTest, 0, Payload{Subject: "x"}, EnqueueOptions{})
	if id != 0 {
		t.Fatalf("expected enqueue to skip disabled channel (id=0), got %d", id)
	}
}

func TestDispatcherNotifyChecksDedupesChannels(t *testing.T) {
	sender := &recordingSender{}
	d, channel := newDispatcherForTest(t, ChannelEmail, 3, 1, sender)
	ctx := context.Background()

	check1, _ := d.q.CreateCheck(ctx, store.CreateCheckParams{
		Name: "c1", Type: "http", Enabled: true,
		IntervalSeconds: 60, TimeoutSeconds: 10, FailureThreshold: 1, RecoveryThreshold: 1,
		ConfigJson: `{"url":"https://example.com"}`,
	})
	check2, _ := d.q.CreateCheck(ctx, store.CreateCheckParams{
		Name: "c2", Type: "http", Enabled: true,
		IntervalSeconds: 60, TimeoutSeconds: 10, FailureThreshold: 1, RecoveryThreshold: 1,
		ConfigJson: `{"url":"https://example.com"}`,
	})

	for _, cid := range []int64{check1.ID, check2.ID} {
		if err := d.q.LinkCheckToChannel(ctx, store.LinkCheckToChannelParams{
			CheckID: cid, ChannelID: channel.ID,
		}); err != nil {
			t.Fatalf("link: %v", err)
		}
	}

	count, err := d.NotifyChecks(ctx, EventIncidentOpened, 1,
		Payload{Subject: "down"}, []int64{check1.ID, check2.ID})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unique delivery, got %d", count)
	}
}
