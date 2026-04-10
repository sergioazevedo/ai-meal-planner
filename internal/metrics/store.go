package metrics

import (
	"ai-meal-planner/internal/metrics/metrics_db"
	"ai-meal-planner/internal/shared"
	"context"
	"database/sql"
	"time"
)

// ExecutionMetric records metadata for a single agent execution.
type ExecutionMetric struct {
	AgentName        string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int64
	Timestamp        time.Time
	ToolCalls        []shared.ToolCallMeta
}

// Store handles persistence of metrics to SQLite.
type Store struct {
	queries *metricsdb.Queries
	db      *sql.DB
}

// NewStore initializes the Store with an existing database connection.
func NewStore(db *sql.DB) *Store {
	return &Store{
		queries: metricsdb.New(db),
		db:      db,
	}
}

// Record saves a metric and its tool calls to the database.
func (s *Store) Record(m ExecutionMetric) error {
	ts := m.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	ctx := context.Background()
	// Using a transaction to ensure atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := s.queries.WithTx(tx)

	metricID, err := qtx.InsertExecutionMetric(ctx, metricsdb.InsertExecutionMetricParams{
		AgentName:        m.AgentName,
		Model:            m.Model,
		PromptTokens:     int64(m.PromptTokens),
		CompletionTokens: int64(m.CompletionTokens),
		LatencyMs:        m.LatencyMS,
		Timestamp:        ts,
	})
	if err != nil {
		return err
	}

	// Aggregate tool calls by name
	toolCounts := make(map[string]int64)
	toolLatency := make(map[string]int64)
	for _, tc := range m.ToolCalls {
		toolCounts[tc.ToolName]++
		toolLatency[tc.ToolName] += tc.Latency.Milliseconds()
	}

	for name, count := range toolCounts {
		err = qtx.InsertExecutionToolCall(ctx, metricsdb.InsertExecutionToolCallParams{
			ExecutionMetricID: metricID,
			ToolName:          name,
			CallCount:         count,
			TotalLatencyMs:    toolLatency[name],
		})
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RecordMeta records metrics directly from shared.AgentMeta.
func (s *Store) RecordMeta(meta shared.AgentMeta) error {
	if meta.Usage.PromptTokens == 0 && meta.Usage.CompletionTokens == 0 {
		return nil
	}
	return s.Record(MapUsage(meta))
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DailyUsage represents token totals for a single day.
type DailyUsage struct {
	Date            string
	TotalPrompt     int
	TotalCompletion int
	TotalExecution  int
}

// GetDailyUsage retrieves usage for the last N days.
func (s *Store) GetDailyUsage(days int) ([]DailyUsage, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	rows, err := s.queries.GetDailyUsage(context.Background(), since)
	if err != nil {
		return nil, err
	}

	var results []DailyUsage
	for _, r := range rows {
		u := DailyUsage{
			TotalExecution: int(r.Count),
		}

		if day, ok := r.Day.(string); ok {
			u.Date = day
		} else {
			u.Date = "Unknown"
		}

		if r.Sum.Valid {
			u.TotalPrompt = int(r.Sum.Float64)
		}
		if r.Sum_2.Valid {
			u.TotalCompletion = int(r.Sum_2.Float64)
		}

		results = append(results, u)
	}
	return results, nil
}

// Cleanup removes records older than the specified number of days.
func (s *Store) Cleanup(olderThanDays int) (int64, error) {
	threshold := time.Now().AddDate(0, 0, -olderThanDays)
	err := s.queries.CleanupExecutionMetrics(context.Background(), threshold)
	if err != nil {
		return 0, err
	}

	// sqlc's :exec doesn't return rows affected for SQLite easily without extra steps.
	// For simplicity, we'll return 0 or implement a custom check if needed.
	return 0, nil
}

// MapUsage helper to convert llm.TokenUsage to ExecutionMetric.
func MapUsage(meta shared.AgentMeta) ExecutionMetric {
	return ExecutionMetric{
		AgentName:        meta.AgentName,
		Model:            meta.Usage.Model,
		PromptTokens:     meta.Usage.PromptTokens,
		CompletionTokens: meta.Usage.CompletionTokens,
		LatencyMS:        meta.Latency.Milliseconds(),
		Timestamp:        time.Now().UTC(),
		ToolCalls:        meta.ToolCalls,
	}
}
