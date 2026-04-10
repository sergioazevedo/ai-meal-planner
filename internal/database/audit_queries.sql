-- name: InsertAuditLog :exec
INSERT INTO audit_logs (
    user_id, plan_id, action_type, original_request, user_feedback, previous_state, new_state
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CleanupAuditLogs :exec
DELETE FROM audit_logs WHERE created_at < ?;
