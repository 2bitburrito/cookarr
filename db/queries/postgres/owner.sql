-- name: CountOwners :one
SELECT COUNT(*) FROM owner;

-- name: CreateOwner :exec
INSERT INTO owner (id, username, password_hash, created_at)
VALUES (1, $1, $2, $3);

-- name: GetOwner :one
SELECT username, password_hash, created_at FROM owner WHERE id = 1;

-- name: SetPassword :execrows
UPDATE owner SET password_hash = $1 WHERE id = 1;
