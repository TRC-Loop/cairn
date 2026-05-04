-- name: ListChecksForIncident :many
SELECT c.*
FROM checks c
JOIN incident_affected_checks iac ON iac.check_id = c.id
WHERE iac.incident_id = ?
ORDER BY c.name ASC;

-- name: ListIncidentsForCheck :many
SELECT i.*
FROM incidents i
JOIN incident_affected_checks iac ON iac.incident_id = i.id
WHERE iac.check_id = ?
ORDER BY i.started_at DESC
LIMIT ?;

-- name: LinkCheckToIncident :exec
INSERT OR IGNORE INTO incident_affected_checks (incident_id, check_id)
VALUES (?, ?);

-- name: UnlinkCheckFromIncident :exec
DELETE FROM incident_affected_checks
WHERE incident_id = ? AND check_id = ?;

-- name: ListAffectedCheckIDsForIncident :many
SELECT check_id FROM incident_affected_checks
WHERE incident_id = ?;
