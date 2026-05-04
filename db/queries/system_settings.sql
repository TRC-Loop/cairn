-- name: GetSystemSettings :one
SELECT * FROM system_settings WHERE id = 1 LIMIT 1;

-- name: UpdateSystemSettings :exec
UPDATE system_settings
SET incident_id_format             = ?,
    incident_reopen_window_seconds = ?,
    incident_reopen_mode           = ?,
    updated_at                     = CURRENT_TIMESTAMP
WHERE id = 1;
