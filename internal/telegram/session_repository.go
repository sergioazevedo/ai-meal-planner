package telegram

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	sessiondb "ai-meal-planner/internal/telegram/session_db"
)

// Session represents an active user session (e.g., awaiting adjustment feedback)
type Session struct {
	ID          int64
	UserID      string
	SessionType string
	State       string
	ContextData string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// SessionContextData holds structured data stored in the context_data JSON field
type SessionContextData struct {
	PlanID          int64  `json:"plan_id"`
	OriginalRequest string `json:"original_request"`
}

// SessionRepository provides access to session persistence operations
type SessionRepository struct {
	queries *sessiondb.Queries
	db      *sql.DB
}

// NewSessionRepository creates a new SessionRepository instance
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{
		queries: sessiondb.New(db),
		db:      db,
	}
}

// Create creates a new session and returns its ID
func (sr *SessionRepository) Create(ctx context.Context, userID, sessionType, state string, contextData SessionContextData, ttlSeconds int) (int64, error) {
	jsonData, err := json.Marshal(contextData)
	if err != nil {
		return 0, err
	}

	expiresAt := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	now := time.Now()

	result, err := sr.queries.CreateSession(ctx, sessiondb.CreateSessionParams{
		UserID:      userID,
		SessionType: sessionType,
		State:       state,
		ContextData: string(jsonData),
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	})
	if err != nil {
		return 0, err
	}

	return result, nil
}

// GetActive retrieves the most recent active session for a user (non-expired)
func (sr *SessionRepository) GetActive(ctx context.Context, userID string, now time.Time) (*Session, error) {
	row, err := sr.queries.GetActiveSession(ctx, sessiondb.GetActiveSessionParams{
		UserID:    userID,
		ExpiresAt: now,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &Session{
		ID:          row.ID,
		UserID:      row.UserID,
		SessionType: row.SessionType,
		State:       row.State,
		ContextData: row.ContextData,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// GetContextData unmarshals the context_data JSON field
func (s *Session) GetContextData() (SessionContextData, error) {
	var data SessionContextData
	err := json.Unmarshal([]byte(s.ContextData), &data)
	return data, err
}

// Update updates the state and context_data for a session
func (sr *SessionRepository) Update(ctx context.Context, sessionID int64, state string, contextData SessionContextData) error {
	jsonData, err := json.Marshal(contextData)
	if err != nil {
		return err
	}

	return sr.queries.UpdateSession(ctx, sessiondb.UpdateSessionParams{
		State:       state,
		ContextData: string(jsonData),
		ID:          sessionID,
	})
}

// Delete removes a session
func (sr *SessionRepository) Delete(ctx context.Context, sessionID int64) error {
	return sr.queries.DeleteSession(ctx, sessionID)
}

// CleanupExpired removes all expired sessions (optional maintenance task)
func (sr *SessionRepository) CleanupExpired(ctx context.Context) error {
	return sr.queries.CleanupExpiredSessions(ctx, time.Now())
}
