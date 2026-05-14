-- name: ListIncidentsSince :many
SELECT * FROM incidents WHERE started_at >= ? ORDER BY started_at DESC;

-- name: DashboardUptime24h :one
SELECT
  CAST(COALESCE(SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END), 0) AS INTEGER) AS up_count,
  CAST(COALESCE(COUNT(*), 0) AS INTEGER) AS total_count
FROM check_results
WHERE checked_at >= ?;
