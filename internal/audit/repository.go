package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ai-meal-planner/internal/audit/db"
)

// AuditRepository handles persistence of audit logs for LLM interactions.
type AuditRepository struct {
	queries *auditdb.Queries
	db      *sql.DB
}

// NewAuditRepository creates a new AuditRepository.
func NewAuditRepository(d *sql.DB) *AuditRepository {
	return &AuditRepository{
		queries: auditdb.New(d),
		db:      d,
	}
}

// LogInteraction records a snapshot of a plan adjustment.
func (r *AuditRepository) LogInteraction(
	ctx context.Context,
	userID string,
	planID int64,
	actionType string,
	originalRequest string,
	userFeedback string,
	prevState interface{},
	newState interface{},
) error {
	prevJSON, err := json.Marshal(prevState)
	if err != nil {
		return fmt.Errorf("failed to marshal previous state: %w", err)
	}

	newJSON, err := json.Marshal(newState)
	if err != nil {
		return fmt.Errorf("failed to marshal new state: %w", err)
	}

	params := auditdb.InsertAuditLogParams{
		UserID:          userID,
		PlanID:          sql.NullInt64{Int64: planID, Valid: true},
		ActionType:      actionType,
		OriginalRequest: sql.NullString{String: originalRequest, Valid: originalRequest != ""},
		UserFeedback:    sql.NullString{String: userFeedback, Valid: userFeedback != ""},
		PreviousState:   sql.NullString{String: string(prevJSON), Valid: true},
		NewState:        sql.NullString{String: string(newJSON), Valid: true},
	}

	return r.queries.InsertAuditLog(ctx, params)
}

// Cleanup removes audit logs older than the specified number of days.
func (r *AuditRepository) Cleanup(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.queries.CleanupAuditLogs(ctx, cutoff)
}
