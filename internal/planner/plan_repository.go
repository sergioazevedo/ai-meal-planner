package planner

import (
	db "ai-meal-planner/internal/planner/plan_db"
	"context"
	"database/sql"
	"fmt"
)

// PlanRepository is a database-backed repository for meal plans.
type PlanRepository struct {
	queries *db.Queries
	db      *sql.DB
}

// NewPlanRepository creates a new PlanRepository.
func NewPlanRepository(d *sql.DB) *PlanRepository {
	return &PlanRepository{
		queries: db.New(d),
		db:      d,
	}
}

// Save inserts a new meal plan into the database.
func (r *PlanRepository) Save(ctx context.Context, userID string, planData []byte) error {
	params := db.InsertMealPlanParams{
		UserID:   userID,
		PlanData: string(planData),
	}
	return r.queries.InsertMealPlan(ctx, params)
}

// ListRecentByUserID retrieves the N most recent meal plans for a given user.
func (r *PlanRepository) ListRecentByUserID(ctx context.Context, userID string, limit int) ([]MealPlan, error) {
	dbPlans, err := r.queries.ListRecentMealPlansByUserID(ctx, db.ListRecentMealPlansByUserIDParams{
		UserID: userID,
		Limit:  int64(limit), // sqlc generates int32 for LIMIT
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list recent meal plans for user %s: %w", userID, err)
	}

	var mealPlans []MealPlan
	for _, dbPlan := range dbPlans {
		mealPlans = append(mealPlans, MealPlan{
			ID:       dbPlan.ID,
			UserID:   dbPlan.UserID,
			PlanData: dbPlan.PlanData,
		})
	}
	return mealPlans, nil
}
