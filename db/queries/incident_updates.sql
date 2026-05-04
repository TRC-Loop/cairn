-- name: ListUpdatesForIncident :many
SELECT * FROM incident_updates
WHERE incident_id = ?
ORDER BY created_at ASC;

-- name: CreateIncidentUpdate :one
INSERT INTO incident_updates (
    incident_id, status, message, posted_by_user_id, auto_generated
) VALUES (
    ?, ?, ?, ?, ?
)
RETURNING *;
