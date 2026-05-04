-- name: CreateRecoveryCode :one
INSERT INTO recovery_codes (user_id, code_hash)
VALUES (?, ?)
RETURNING *;

-- name: ListUnusedRecoveryCodesForUser :many
SELECT * FROM recovery_codes
WHERE user_id = ? AND used_at IS NULL
ORDER BY id ASC;

-- name: MarkRecoveryCodeUsed :exec
UPDATE recovery_codes SET used_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: DeleteRecoveryCodesForUser :exec
DELETE FROM recovery_codes WHERE user_id = ?;

-- name: CountUnusedRecoveryCodes :one
SELECT COUNT(*) FROM recovery_codes
WHERE user_id = ? AND used_at IS NULL;
