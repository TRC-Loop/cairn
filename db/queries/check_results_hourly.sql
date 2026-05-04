-- name: UpsertCheckResultHourly :exec
INSERT INTO check_results_hourly (
    check_id, hour_bucket, total_count, up_count, degraded_count,
    down_count, unknown_count, avg_latency_ms, min_latency_ms,
    max_latency_ms, p95_latency_ms
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
ON CONFLICT(check_id, hour_bucket) DO UPDATE SET
    total_count    = excluded.total_count,
    up_count       = excluded.up_count,
    degraded_count = excluded.degraded_count,
    down_count     = excluded.down_count,
    unknown_count  = excluded.unknown_count,
    avg_latency_ms = excluded.avg_latency_ms,
    min_latency_ms = excluded.min_latency_ms,
    max_latency_ms = excluded.max_latency_ms,
    p95_latency_ms = excluded.p95_latency_ms;

-- name: GetHourlyInRange :many
SELECT * FROM check_results_hourly
WHERE check_id = ?
  AND hour_bucket >= ?
  AND hour_bucket <= ?
ORDER BY hour_bucket ASC;

-- name: DeleteHourlyOlderThan :exec
DELETE FROM check_results_hourly WHERE hour_bucket < ?;

-- name: GetRawResultsForRollup :many
SELECT * FROM check_results
WHERE check_id = ?
  AND checked_at >= ?
  AND checked_at < ?
ORDER BY checked_at ASC;

-- name: ListChecksWithHourly :many
SELECT DISTINCT check_id FROM check_results_hourly;

-- name: GetOldestHourly :one
SELECT MIN(hour_bucket) AS oldest FROM check_results_hourly WHERE check_id = ?;
