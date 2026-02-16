-- name: CreateSession :one
INSERT INTO user_sessions (user_id, session_type, state, context_data, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: GetActiveSession :one
SELECT id, user_id, session_type, state, context_data, expires_at, created_at
FROM user_sessions
WHERE user_id = ? AND expires_at > ?
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateSession :exec
UPDATE user_sessions
SET state = ?, context_data = ?
WHERE id = ?;

-- name: DeleteSession :exec
DELETE FROM user_sessions WHERE id = ?;

-- name: CleanupExpiredSessions :exec
DELETE FROM user_sessions WHERE expires_at <= ?;
