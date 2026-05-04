-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ? LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (username, email, display_name, password_hash, role)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users ORDER BY created_at ASC;

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: CountUsersByRole :one
SELECT COUNT(*) FROM users WHERE role = ?;

-- name: UpdateUserRole :exec
UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: UpdateUserMetadata :exec
UPDATE users
SET email = ?, display_name = ?, role = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- name: SetUserTOTPSecret :exec
UPDATE users
SET totp_secret_enc = ?, totp_enabled = 0, totp_enrolled_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: EnableUserTOTP :exec
UPDATE users
SET totp_enabled = 1, totp_enrolled_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DisableUserTOTP :exec
UPDATE users
SET totp_secret_enc = NULL, totp_enabled = 0, totp_enrolled_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;
