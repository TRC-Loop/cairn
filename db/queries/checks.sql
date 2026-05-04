-- name: GetCheck :one
SELECT * FROM checks WHERE id = ? LIMIT 1;

-- name: ListChecks :many
SELECT * FROM checks ORDER BY name ASC;

-- name: ListEnabledChecksDue :many
SELECT * FROM checks
WHERE enabled = 1
  AND type != 'push'
  AND (last_checked_at IS NULL
       OR last_checked_at <= datetime('now', printf('-%d seconds', interval_seconds)))
ORDER BY last_checked_at ASC NULLS FIRST;

-- name: GetCheckByPushToken :one
SELECT * FROM checks WHERE push_token = ? LIMIT 1;

-- name: SetCheckPushToken :exec
UPDATE checks SET push_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: ListEnabledPushChecks :many
SELECT * FROM checks
WHERE enabled = 1
  AND type = 'push'
ORDER BY id ASC;

-- name: CreateCheck :one
INSERT INTO checks (
    name, type, enabled, interval_seconds, timeout_seconds, retries,
    failure_threshold, recovery_threshold, config_json, component_id,
    reopen_window_seconds, reopen_mode
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateCheck :one
UPDATE checks
SET name                  = ?,
    type                  = ?,
    enabled               = ?,
    interval_seconds      = ?,
    timeout_seconds       = ?,
    retries               = ?,
    failure_threshold     = ?,
    recovery_threshold    = ?,
    config_json           = ?,
    component_id          = ?,
    reopen_window_seconds = ?,
    reopen_mode           = ?,
    updated_at            = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: UpdateCheckStatus :exec
UPDATE checks
SET last_status           = ?,
    last_latency_ms       = ?,
    last_checked_at       = ?,
    consecutive_failures  = ?,
    consecutive_successes = ?,
    updated_at            = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteCheck :exec
DELETE FROM checks WHERE id = ?;

-- name: CountChecksByStatus :many
SELECT last_status AS status, COUNT(*) AS count
FROM checks
WHERE enabled = 1
GROUP BY last_status;
