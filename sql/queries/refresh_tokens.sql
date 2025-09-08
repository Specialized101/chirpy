-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens
(token, created_at, updated_at, expires_at, revoked_at, user_id)
VALUES
($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetRefreshToken :one
SELECT *
FROM refresh_tokens
WHERE token = $1;

-- name: GetRefreshTokenByUserId :one
SELECT *
FROM refresh_tokens
WHERE user_id = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET updated_at = NOW(),
    revoked_at = NOW()
WHERE token = $1;