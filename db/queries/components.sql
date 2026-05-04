-- name: GetComponent :one
SELECT * FROM components WHERE id = ? LIMIT 1;

-- name: ListComponents :many
SELECT * FROM components ORDER BY display_order ASC, name ASC;

-- name: CreateComponent :one
INSERT INTO components (name, description, display_order)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateComponent :exec
UPDATE components
SET name          = ?,
    description   = ?,
    display_order = ?,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteComponent :exec
DELETE FROM components WHERE id = ?;

-- name: ListChecksForComponent :many
SELECT checks.* FROM checks
WHERE checks.component_id = ?
ORDER BY checks.name ASC;

-- name: CountChecksForComponent :one
SELECT COUNT(*) AS count FROM checks WHERE component_id = ?;
