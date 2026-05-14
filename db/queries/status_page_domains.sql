-- name: ListDomainsForStatusPage :many
SELECT * FROM status_page_domains
WHERE status_page_id = ?
ORDER BY domain ASC;

-- name: ListAllStatusPageDomains :many
SELECT * FROM status_page_domains ORDER BY domain ASC;

-- name: GetStatusPageDomain :one
SELECT * FROM status_page_domains WHERE id = ? LIMIT 1;

-- name: AddStatusPageDomain :one
INSERT INTO status_page_domains (status_page_id, domain)
VALUES (?, ?)
RETURNING *;

-- name: DeleteStatusPageDomain :exec
DELETE FROM status_page_domains WHERE id = ? AND status_page_id = ?;

-- name: LookupStatusPageByDomain :one
SELECT status_pages.*
FROM status_pages
JOIN status_page_domains ON status_page_domains.status_page_id = status_pages.id
WHERE status_page_domains.domain = ?
LIMIT 1;

-- name: CountAllStatusPageDomains :one
SELECT COUNT(*) FROM status_page_domains;
