-- name: CreateSession :one
INSERT INTO sessions (id, user_id, expires_at, user_agent, ip_address)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSession :one
SELECT * FROM sessions WHERE id = ? LIMIT 1;

-- name: GetSessionWithUser :one
SELECT sqlc.embed(sessions), sqlc.embed(users)
FROM sessions
INNER JOIN users ON users.id = sessions.user_id
WHERE sessions.id = ? LIMIT 1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = ?;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = ?;

-- name: DeleteSessionsForUserExcept :exec
DELETE FROM sessions WHERE user_id = ? AND id != ?;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= datetime('now');

-- name: ListSessionsForUser :many
SELECT * FROM sessions WHERE user_id = ? ORDER BY created_at DESC;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = ? WHERE id = ?;
