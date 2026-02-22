-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByGoogleID :one
SELECT * FROM users
WHERE google_id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY id desc;

-- name: CreateUser :one
INSERT INTO users (
    name,
    email,
    password,
    uuid,
    google_id,
    profile_image_url,
    is_super_admin,
    created_at,
    updated_at,
    deleted_at,
    settings
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    profile_image_url= $3,
    google_id = CASE WHEN @set_google_id::boolean THEN $4 ELSE users.google_id END,
    updated_at = $5
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :one
UPDATE users
SET password = $2,
    updated_at = $3
WHERE id = $1
RETURNING *;


-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: MarkUserAsSuperAdmin :exec
UPDATE users
SET is_super_admin = true, updated_at = NOW()
WHERE id = $1;

-- name: GetSuperAdmins :many
SELECT * FROM users
WHERE is_super_admin = true
AND deleted_at is null
ORDER BY id ASC;