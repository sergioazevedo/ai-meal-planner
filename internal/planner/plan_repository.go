package planner

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"ai-meal-planner/internal/planner/plan_db"
)

// MealPlan represents a stored meal plan.
type MealPlan struct {
	ID        int32
	UserID    string
	PlanData  []byte // Raw JSON of the meal plan
	CreatedAt time.Time
}

// PlanRepository is a database-backed repository for meal plans.
type PlanRepository struct {
	queries *plan_db.Queries
	db      *sql.DB
}

// NewPlanRepository creates a new PlanRepository.
func NewPlanRepository(d *sql.DB) *PlanRepository {
	return &PlanRepository{
		queries: plan_db.New(d),
		db:      d,
	}
}

// Save inserts a new meal plan into the database.
func (r *PlanRepository) Save(ctx context.Context, userID string, planData []byte) error {
	params := plan_db.InsertMealPlanParams{
		UserID:    userID,
		PlanData:  planData,
		CreatedAt: time.Now(),
	}
	return r.queries.InsertMealPlan(ctx, params)
}

// ListRecentByUserID retrieves the N most recent meal plans for a given user.
func (r *PlanRepository) ListRecentByUserID(ctx context.Context, userID string, limit int) ([]MealPlan, error) {
	dbPlans, err := r.queries.ListRecentMealPlansByUserID(ctx, plan_db.ListRecentMealPlansByUserIDParams{
		UserID: userID,
		Limit:  int32(limit), // sqlc generates int32 for LIMIT
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list recent meal plans for user %s: %w", userID, err)
	}

	var mealPlans []MealPlan
	for _, dbPlan := range dbPlans {
		mealPlans = append(mealPlans, MealPlan{
			ID:        dbPlan.ID,
			UserID:    dbPlan.UserID,
			PlanData:  dbPlan.PlanData,
			CreatedAt: dbPlan.CreatedAt,
		})
	}
	return mealPlans, nil
}
