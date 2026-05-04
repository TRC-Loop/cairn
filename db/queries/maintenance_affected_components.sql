-- name: ListAffectedComponents :many
SELECT components.*
FROM maintenance_affected_components
JOIN components ON components.id = maintenance_affected_components.component_id
WHERE maintenance_affected_components.maintenance_id = ?
ORDER BY components.display_order ASC, components.name ASC;

-- name: ListMaintenanceForComponent :many
SELECT maintenance_windows.*
FROM maintenance_affected_components
JOIN maintenance_windows ON maintenance_windows.id = maintenance_affected_components.maintenance_id
WHERE maintenance_affected_components.component_id = ?
  AND maintenance_windows.state = 'in_progress'
ORDER BY maintenance_windows.starts_at ASC;

-- name: LinkComponent :exec
INSERT OR IGNORE INTO maintenance_affected_components (maintenance_id, component_id)
VALUES (?, ?);

-- name: UnlinkComponent :exec
DELETE FROM maintenance_affected_components
WHERE maintenance_id = ? AND component_id = ?;

-- name: ListComponentsForMaintenance :many
SELECT component_id FROM maintenance_affected_components
WHERE maintenance_id = ?;
