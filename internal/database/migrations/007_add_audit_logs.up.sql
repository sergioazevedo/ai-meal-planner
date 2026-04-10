CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    plan_id INTEGER,           -- Associates the log with a specific meal plan thread
    action_type TEXT NOT NULL, -- e.g., "adjust_plan"
    original_request TEXT,
    user_feedback TEXT,        -- The specific adjustment requested
    previous_state TEXT,       -- JSON snapshot of the plan BEFORE
    new_state TEXT,            -- JSON snapshot of the plan AFTER
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX idx_audit_logs_plan_id ON audit_logs(plan_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
