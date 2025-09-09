-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: DeleteUsers :exec
DELETE FROM USERS;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET email = $1,
    hashed_password = $2
WHERE id = $3
RETURNING *;