-- name: CreateSession :exec
INSERT INTO session (token_hash, created_at, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionExpiry :one
SELECT expires_at FROM session WHERE token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM session WHERE token_hash = $1;

-- name: DeleteAllSessions :exec
DELETE FROM session;

-- name: DeleteExpiredSessions :exec
DELETE FROM session WHERE expires_at <= sqlc.arg(now)::timestamptz;
