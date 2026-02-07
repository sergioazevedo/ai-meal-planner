-- name: InsertExecutionMetric :exec
INSERT INTO execution_metrics (agent_name, model, prompt_tokens, completion_tokens, latency_ms, timestamp)
VALUES (?, ?, ?, ?, ?, ?);

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
