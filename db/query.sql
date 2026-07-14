-- name: CreateUser :one
INSERT INTO users (name, email, created_at)
VALUES (sqlc.arg(name), sqlc.arg(email), sqlc.arg(created_at))
ON CONFLICT (email) DO NOTHING
RETURNING id, name, email, created_at;

-- name: GetUser :one
SELECT id, name, email, created_at
FROM users
WHERE id = sqlc.arg(id);

-- name: ListUsers :many
SELECT id, name, email, created_at
FROM users
ORDER BY id
LIMIT sqlc.arg(page_size) OFFSET sqlc.arg(page_offset);

-- name: ListAllUsers :many
SELECT id, name, email, created_at
FROM users
ORDER BY id;

-- name: CountUsers :one
SELECT count(*)
FROM users;
