-- name: ListComponentsForStatusPage :many
SELECT components.*
FROM status_page_components
JOIN components ON components.id = status_page_components.component_id
WHERE status_page_components.status_page_id = ?
ORDER BY status_page_components.display_order ASC;

-- name: AddComponentToStatusPage :exec
INSERT INTO status_page_components (status_page_id, component_id, display_order)
VALUES (?, ?, ?);

-- name: RemoveComponentFromStatusPage :exec
DELETE FROM status_page_components WHERE status_page_id = ? AND component_id = ?;

-- name: UpdateStatusPageComponentOrder :exec
UPDATE status_page_components
SET display_order = ?
WHERE status_page_id = ? AND component_id = ?;

-- name: RemoveAllComponentsFromStatusPage :exec
DELETE FROM status_page_components WHERE status_page_id = ?;

-- name: UpdateStatusPageComponentShowMonitors :exec
UPDATE status_page_components
SET show_monitors_default = ?
WHERE status_page_id = ? AND component_id = ?;

-- name: ListStatusPageComponentSettings :many
SELECT component_id, display_order, show_monitors_default
FROM status_page_components
WHERE status_page_id = ?
ORDER BY display_order ASC;
