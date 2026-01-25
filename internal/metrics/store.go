package metrics

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-meal-planner/internal/llm"

	_ "modernc.org/sqlite"
)

// ExecutionMetric records metadata for a single agent execution.
type ExecutionMetric struct {
	AgentName        string
	Model            string
	PromptTokens     int
	CompletionTokens int
	LatencyMS        int64
	Timestamp        time.Time
}

// Store handles persistence of metrics to SQLite.
type Store struct {
	db *sql.DB
}

// NewStore initializes the SQLite database and creates the metrics table.
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metrics directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS execution_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_name TEXT,
		model TEXT,
		prompt_tokens INTEGER,
		completion_tokens INTEGER,
		latency_ms INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON execution_metrics(timestamp);
	`
	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to migrate metrics db: %w", err)
	}
	return nil
}

// Record saves a metric to the database.
func (s *Store) Record(m ExecutionMetric) error {
	query := `
	INSERT INTO execution_metrics (agent_name, model, prompt_tokens, completion_tokens, latency_ms, timestamp)
	VALUES (?, ?, ?, ?, ?, ?);
	`
	ts := m.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	_, err := s.db.Exec(query, m.AgentName, m.Model, m.PromptTokens, m.CompletionTokens, m.LatencyMS, ts)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DailyUsage represents token totals for a single day.
type DailyUsage struct {
	Date             string
	TotalPrompt      int
	TotalCompletion  int
	TotalExecution   int
}

// GetDailyUsage retrieves usage for the last N days.
func (s *Store) GetDailyUsage(days int) ([]DailyUsage, error) {
	query := `
	SELECT 
		DATE(timestamp) as day,
		SUM(prompt_tokens),
		SUM(completion_tokens),
		COUNT(*)
	FROM execution_metrics
	WHERE timestamp > ?
	GROUP BY day
	ORDER BY day DESC;
	`
	since := time.Now().AddDate(0, 0, -days)
	rows, err := s.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DailyUsage
	for rows.Next() {
		var u DailyUsage
		if err := rows.Scan(&u.Date, &u.TotalPrompt, &u.TotalCompletion, &u.TotalExecution); err != nil {
			return nil, err
		}
		results = append(results, u)
	}
	return results, nil
}

// Cleanup removes records older than the specified number of days.
func (s *Store) Cleanup(olderThanDays int) (int64, error) {
	query := `DELETE FROM execution_metrics WHERE timestamp < ?;`
	threshold := time.Now().AddDate(0, 0, -olderThanDays)
	res, err := s.db.Exec(query, threshold)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// MapUsage helper to convert llm.TokenUsage to ExecutionMetric.
func MapUsage(agentName string, usage llm.TokenUsage, latency time.Duration) ExecutionMetric {
	return ExecutionMetric{
		AgentName:        agentName,
		Model:            usage.Model,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		LatencyMS:        latency.Milliseconds(),
		Timestamp:        time.Now().UTC(),
	}
}
