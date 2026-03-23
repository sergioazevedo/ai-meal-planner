CREATE TABLE IF NOT EXISTS execution_tool_calls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    execution_metric_id INTEGER NOT NULL,
    tool_name TEXT NOT NULL,
    call_count INTEGER NOT NULL DEFAULT 1,
    total_latency_ms INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (execution_metric_id) REFERENCES execution_metrics(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_execution_tool_calls_metric_id ON execution_tool_calls(execution_metric_id);
CREATE INDEX IF NOT EXISTS idx_execution_tool_calls_tool_name ON execution_tool_calls(tool_name);
