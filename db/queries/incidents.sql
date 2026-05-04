-- name: GetIncident :one
SELECT * FROM incidents WHERE id = ? LIMIT 1;

-- name: ListIncidents :many
SELECT * FROM incidents
ORDER BY started_at DESC
LIMIT ? OFFSET ?;

-- name: ListActiveIncidents :many
SELECT * FROM incidents
WHERE status != 'resolved'
ORDER BY started_at DESC;

-- name: ListRecentResolvedIncidents :many
SELECT * FROM incidents
WHERE status = 'resolved'
ORDER BY resolved_at DESC
LIMIT ?;

-- name: CreateIncident :one
INSERT INTO incidents (
    title, status, severity, started_at, auto_created, triggering_check_id, created_by_user_id
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateIncidentStatus :exec
UPDATE incidents
SET status      = ?,
    resolved_at = ?,
    updated_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateIncidentMetadata :exec
UPDATE incidents
SET title      = ?,
    severity   = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteIncident :exec
DELETE FROM incidents WHERE id = ?;

-- name: GetMostRecentResolvedIncidentForCheck :one
SELECT i.*
FROM incidents i
JOIN incident_affected_checks iac ON iac.incident_id = i.id
WHERE iac.check_id = ?
  AND i.status = 'resolved'
  AND i.resolved_at IS NOT NULL
ORDER BY i.resolved_at DESC
LIMIT 1;
