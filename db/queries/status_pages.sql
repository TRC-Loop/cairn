-- name: GetStatusPage :one
SELECT * FROM status_pages WHERE id = ? LIMIT 1;

-- name: GetStatusPageBySlug :one
SELECT * FROM status_pages WHERE slug = ? LIMIT 1;

-- name: GetDefaultStatusPage :one
SELECT * FROM status_pages WHERE is_default = 1 LIMIT 1;

-- name: ListStatusPages :many
SELECT * FROM status_pages ORDER BY is_default DESC, title ASC;

-- name: CreateStatusPage :one
INSERT INTO status_pages (
    slug, title, description, logo_url, accent_color, custom_footer_html, password_hash, is_default
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateStatusPage :exec
UPDATE status_pages
SET title              = ?,
    description        = ?,
    logo_url           = ?,
    accent_color       = ?,
    custom_footer_html = ?,
    updated_at         = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateStatusPagePassword :exec
UPDATE status_pages
SET password_hash = ?,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateStatusPageFooterMode :exec
UPDATE status_pages
SET footer_mode = ?,
    updated_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateStatusPageFlags :exec
UPDATE status_pages
SET hide_powered_by = ?,
    show_history    = ?,
    updated_at      = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: SetStatusPageAsDefault :exec
UPDATE status_pages SET is_default = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UnsetAllDefaults :exec
UPDATE status_pages SET is_default = 0, updated_at = CURRENT_TIMESTAMP WHERE is_default = 1;

-- name: DeleteStatusPage :exec
DELETE FROM status_pages WHERE id = ?;
