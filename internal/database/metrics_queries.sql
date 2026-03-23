-- name: InsertExecutionMetric :one
INSERT INTO execution_metrics (agent_name, model, prompt_tokens, completion_tokens, latency_ms, timestamp)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: InsertExecutionToolCall :exec
INSERT INTO execution_tool_calls (execution_metric_id, tool_name, call_count, total_latency_ms)
VALUES (?, ?, ?, ?);

-- name: GetTurnDistributionByAgent :many
WITH AgentTurns AS (
    SELECT
        m.id,
        m.agent_name,
        COALESCE(SUM(t.call_count), 0) + 1 as turns
    FROM execution_metrics m
    LEFT JOIN execution_tool_calls t ON m.id = t.execution_metric_id
    GROUP BY m.id, m.agent_name
)
SELECT
    agent_name,
    turns,
    COUNT(*) as count
FROM AgentTurns
GROUP BY agent_name, turns
ORDER BY agent_name ASC, turns ASC;

-- name: GetDailyUsage :many
SELECT
    STRFTIME('%Y-%m-%d', timestamp) as day,
    SUM(prompt_tokens),
    SUM(completion_tokens),
    COUNT(*)
FROM execution_metrics
WHERE timestamp > STRFTIME('%Y-%m-%d %H:%M:%S', ?)
GROUP BY day
ORDER BY day DESC;

-- name: CleanupExecutionMetrics :exec
DELETE FROM execution_metrics WHERE timestamp < ?;
