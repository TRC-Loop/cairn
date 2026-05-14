-- name: ListMonitorsForStatusPage :many
SELECT checks.*
FROM status_page_monitors
JOIN checks ON checks.id = status_page_monitors.check_id
WHERE status_page_monitors.status_page_id = ?
ORDER BY status_page_monitors.display_order ASC, checks.id ASC;

-- name: AddMonitorToStatusPage :exec
INSERT INTO status_page_monitors (status_page_id, check_id, display_order)
VALUES (?, ?, ?);

-- name: RemoveAllMonitorsFromStatusPage :exec
DELETE FROM status_page_monitors WHERE status_page_id = ?;
