-- name: GetMaintenanceWindow :one
SELECT * FROM maintenance_windows WHERE id = ? LIMIT 1;

-- name: ListMaintenanceWindows :many
SELECT * FROM maintenance_windows ORDER BY starts_at DESC LIMIT ?;

-- name: ListMaintenanceFiltered :many
SELECT * FROM maintenance_windows
WHERE state = ?
ORDER BY starts_at DESC
LIMIT ? OFFSET ?;

-- name: ListMaintenanceAll :many
SELECT * FROM maintenance_windows
ORDER BY starts_at DESC
LIMIT ? OFFSET ?;

-- name: ListPastMaintenance :many
SELECT * FROM maintenance_windows
WHERE state IN ('completed','cancelled') AND ends_at > ?
ORDER BY ends_at DESC
LIMIT ?;

-- name: CountMaintenanceFiltered :one
SELECT COUNT(*) FROM maintenance_windows WHERE state = ?;

-- name: CountMaintenanceAll :one
SELECT COUNT(*) FROM maintenance_windows;

-- name: CancelMaintenance :execrows
UPDATE maintenance_windows
SET state = 'cancelled', updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND state = 'scheduled';

-- name: EndMaintenanceNow :execrows
UPDATE maintenance_windows
SET state = 'completed', ends_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND state = 'in_progress';

-- name: ListActiveMaintenance :many
SELECT * FROM maintenance_windows WHERE state = 'in_progress' ORDER BY starts_at ASC;

-- name: ListUpcomingMaintenance :many
SELECT * FROM maintenance_windows
WHERE state = 'scheduled' AND starts_at > datetime('now')
ORDER BY starts_at ASC;

-- name: ListMaintenanceBetween :many
SELECT * FROM maintenance_windows
WHERE starts_at < ? AND ends_at > ?
ORDER BY starts_at ASC;

-- name: CreateMaintenanceWindow :one
INSERT INTO maintenance_windows (
    title, description, starts_at, ends_at, state, created_by_user_id
) VALUES (
    ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateMaintenanceWindow :exec
UPDATE maintenance_windows
SET title       = ?,
    description = ?,
    starts_at   = ?,
    ends_at     = ?,
    updated_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateMaintenanceState :exec
UPDATE maintenance_windows
SET state      = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteMaintenanceWindow :exec
DELETE FROM maintenance_windows WHERE id = ?;

-- name: AutoStartDueMaintenance :exec
UPDATE maintenance_windows
SET state = 'in_progress', updated_at = CURRENT_TIMESTAMP
WHERE state = 'scheduled' AND starts_at <= datetime('now');

-- name: AutoCompleteDueMaintenance :exec
UPDATE maintenance_windows
SET state = 'completed', updated_at = CURRENT_TIMESTAMP
WHERE state = 'in_progress' AND ends_at <= datetime('now');
