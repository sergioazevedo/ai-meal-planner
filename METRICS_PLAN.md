# Zero-Cost Observability Plan

This document outlines the strategy for monitoring LLM usage and system health for the AI Meal Planner.

## 1. Storage Strategy
- **Engine**: SQLite (via `modernc.org/sqlite` - pure Go, no CGO required).
- **Location**: `data/db/metrics.db`.
- **Retention**: 30-day rolling window.

## 2. Metrics to Track
For every Agent execution (Analyst, Chef, etc.), we record:
- `timestamp`: UTC execution time.
- `agent_name`: Name of the agent.
- `model`: The LLM model used.
- `prompt_tokens`: Input size (to monitor "Context Bloat").
- `completion_tokens`: Output size.
- `latency_ms`: Execution duration.

## 3. Monitoring & Alerting
- **Admin Command**: `/metrics` (Restricted to `ADMIN_TELEGRAM_ID`).
  - Shows Today's usage vs free-tier limits.
  - Shows 7-day velocity summary.
  - Shows real-time system health (RAM, CPU, Disk).
- **Proactive Alerts**:
  - **Context Bloat**: Single request > 4,000 prompt tokens.
  - **Daily Quota**: Usage > 80% of configured free-tier limit.

## 4. Maintenance
- **Cleanup**: A dedicated CLI command `ai-meal-planner metrics cleanup --days 30`.
- **Automation**: A cron job running nightly to execute the cleanup script.

## 5. Implementation Roadmap
1. [x] Configuration refactor (`AdminTelegramID`, `MetricsDBPath`).
2. [ ] Add SQLite dependencies.
3. [ ] Implement `internal/metrics` package (Store, Record, Summary).
4. [ ] Wire agents to record metrics.
5. [ ] Implement Telegram `/metrics` handler and alerting logic.
6. [ ] Implement CLI cleanup command and cron script.
