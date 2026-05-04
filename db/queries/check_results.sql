-- name: InsertCheckResult :exec
INSERT INTO check_results (
    check_id, checked_at, status, latency_ms, error_message, response_metadata_json
) VALUES (
    ?, ?, ?, ?, ?, ?
);

-- name: GetRecentResults :many
SELECT * FROM check_results
WHERE check_id = ?
ORDER BY checked_at DESC
LIMIT ?;

-- name: GetResultsInRange :many
SELECT * FROM check_results
WHERE check_id = ?
  AND checked_at >= ?
  AND checked_at <= ?
ORDER BY checked_at ASC;

-- name: DeleteResultsOlderThan :exec
DELETE FROM check_results WHERE checked_at < ?;

-- name: CountResultsByStatusInRange :many
SELECT status, COUNT(*) AS count
FROM check_results
WHERE check_id = ?
  AND checked_at >= ?
  AND checked_at <= ?
GROUP BY status;

-- name: ListChecksWithRawResults :many
SELECT DISTINCT check_id FROM check_results;

-- name: GetOldestRawResult :one
SELECT MIN(checked_at) AS oldest FROM check_results WHERE check_id = ?;

-- name: GetMostRecentResult :one
SELECT * FROM check_results
WHERE check_id = ?
ORDER BY checked_at DESC
LIMIT 1;
