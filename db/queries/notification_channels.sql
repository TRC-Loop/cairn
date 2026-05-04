-- name: GetNotificationChannel :one
SELECT * FROM notification_channels WHERE id = ? LIMIT 1;

-- name: ListNotificationChannels :many
SELECT * FROM notification_channels ORDER BY name ASC;

-- name: ListEnabledNotificationChannels :many
SELECT * FROM notification_channels WHERE enabled = 1 ORDER BY name ASC;

-- name: CreateNotificationChannel :one
INSERT INTO notification_channels (
    name, type, enabled, config_json, retry_max, retry_backoff_seconds
) VALUES (
    ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateNotificationChannel :one
UPDATE notification_channels
SET name                  = ?,
    enabled               = ?,
    config_json           = ?,
    retry_max             = ?,
    retry_backoff_seconds = ?,
    updated_at            = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteNotificationChannel :exec
DELETE FROM notification_channels WHERE id = ?;

-- name: CountChecksForChannel :one
SELECT COUNT(*) FROM check_notification_channels WHERE channel_id = ?;
