-- name: UpsertCheckResultDaily :exec
INSERT INTO check_results_daily (
    check_id, day_bucket, total_count, up_count, degraded_count,
    down_count, unknown_count, avg_latency_ms, min_latency_ms,
    max_latency_ms, p95_latency_ms
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(check_id, day_bucket) DO UPDATE SET
    total_count    = excluded.total_count,
    up_count       = excluded.up_count,
    degraded_count = excluded.degraded_count,
    down_count     = excluded.down_count,
    unknown_count  = excluded.unknown_count,
    avg_latency_ms = excluded.avg_latency_ms,
    min_latency_ms = excluded.min_latency_ms,
    max_latency_ms = excluded.max_latency_ms,
    p95_latency_ms = excluded.p95_latency_ms;

-- name: GetDailyInRange :many
SELECT * FROM check_results_daily
WHERE check_id = ?
  AND day_bucket >= ?
  AND day_bucket <= ?
ORDER BY day_bucket ASC;

-- name: DeleteDailyOlderThan :exec
DELETE FROM check_results_daily WHERE day_bucket < ?;

-- name: GetHourlyForRollup :many
SELECT * FROM check_results_hourly
WHERE check_id = ?
  AND hour_bucket >= ?
  AND hour_bucket < ?
ORDER BY hour_bucket ASC;
