-- name: CreateNotificationDelivery :one
INSERT INTO notification_deliveries (
    channel_id, event_type, event_id, payload_json, status, next_attempt_at
) VALUES (
    ?, ?, ?, ?, 'pending', ?
)
RETURNING *;

-- name: GetNotificationDelivery :one
SELECT * FROM notification_deliveries WHERE id = ? LIMIT 1;

-- name: ListPendingDeliveries :many
SELECT * FROM notification_deliveries
WHERE status = 'pending' AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
ORDER BY id ASC
LIMIT ?;

-- name: ListRecentDeliveriesForChannel :many
SELECT * FROM notification_deliveries
WHERE channel_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: MarkDeliverySending :exec
UPDATE notification_deliveries
SET status            = 'sending',
    last_attempted_at = ?
WHERE id = ?;

-- name: MarkDeliverySent :exec
UPDATE notification_deliveries
SET status        = 'sent',
    attempt_count = attempt_count + 1,
    sent_at       = ?,
    last_error    = NULL
WHERE id = ?;

-- name: MarkDeliveryRetry :exec
UPDATE notification_deliveries
SET status          = 'pending',
    attempt_count   = attempt_count + 1,
    last_error      = ?,
    next_attempt_at = ?
WHERE id = ?;

-- name: MarkDeliveryFailed :exec
UPDATE notification_deliveries
SET status        = 'failed',
    attempt_count = attempt_count + 1,
    last_error    = ?
WHERE id = ?;

-- name: CountDeliveriesForEvent :one
SELECT COUNT(*) FROM notification_deliveries
WHERE channel_id = ? AND event_type = ? AND event_id = ?;
