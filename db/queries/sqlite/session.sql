-- name: CreateSession :exec
INSERT INTO session (token_hash, created_at, expires_at) VALUES (?, ?, ?);

-- name: GetSessionExpiry :one
SELECT expires_at FROM session WHERE token_hash = ?;

-- name: DeleteSession :exec
DELETE FROM session WHERE token_hash = ?;

-- name: DeleteAllSessions :exec
DELETE FROM session;

-- name: DeleteExpiredSessions :exec
DELETE FROM session WHERE julianday(expires_at) <= julianday(CAST(sqlc.arg(now) AS TEXT));
