-- 005_add_user_sessions.up.sql
-- Add user_sessions table for tracking conversation state during plan adjustments

CREATE TABLE IF NOT EXISTS user_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    session_type TEXT NOT NULL,
    state TEXT NOT NULL,
    context_data TEXT NOT NULL,  -- JSON with planID, etc.
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active ON user_sessions(user_id, expires_at);
