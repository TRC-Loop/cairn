-- name: GetRetentionSettings :one
SELECT * FROM retention_settings WHERE id = 1 LIMIT 1;

-- name: UpdateRetentionSettings :exec
UPDATE retention_settings
SET raw_days           = ?,
    hourly_days        = ?,
    daily_days         = ?,
    keep_daily_forever = ?,
    updated_at         = CURRENT_TIMESTAMP
WHERE id = 1;
