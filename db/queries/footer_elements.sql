-- name: ListFooterElements :many
SELECT * FROM status_page_footer_elements
WHERE status_page_id = ?
ORDER BY display_order ASC, id ASC;

-- name: CreateFooterElement :one
INSERT INTO status_page_footer_elements (
    status_page_id, element_type, label, url, open_in_new_tab, display_order
) VALUES (
    ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateFooterElement :exec
UPDATE status_page_footer_elements
SET element_type    = ?,
    label           = ?,
    url             = ?,
    open_in_new_tab = ?,
    display_order   = ?
WHERE id = ?;

-- name: DeleteFooterElement :exec
DELETE FROM status_page_footer_elements WHERE id = ?;

-- name: DeleteFooterElementsForPage :exec
DELETE FROM status_page_footer_elements WHERE status_page_id = ?;
