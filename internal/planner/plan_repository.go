package planner

import (
	db "ai-meal-planner/internal/planner/plan_db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
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
func (r *PlanRepository) Save(ctx context.Context, userID string, planData *MealPlan) error {
	planJSON, err := json.Marshal(planData)
	if err != nil {
		log.Printf("Warning: failed to marshal meal plan to JSON for saving: %v", err)
	}

	params := db.InsertMealPlanParams{
		UserID:        userID,
		PlanData:      string(planJSON),
		WeekStartDate: planData.WeekStart,
		CreatedAt:     time.Now().UTC(),
	}
	return r.queries.InsertMealPlan(ctx, params)
}

// ListRecentByUserID retrieves the N most recent meal plans for a given user.
func (r *PlanRepository) ListRecentByUserID(ctx context.Context, userID string, limit int) ([]MealPlan, error) {
	dbPlans, err := r.queries.ListRecentMealPlansByUserID(ctx, db.ListRecentMealPlansByUserIDParams{
		UserID: userID,
		Limit:  int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list recent meal plans for user %s: %w", userID, err)
	}

	var mealPlans []MealPlan
	for _, dbPlan := range dbPlans {
		plan := MealPlan{}
		if err := json.Unmarshal([]byte(dbPlan.PlanData), &plan); err == nil {
			plan.WeekStart = dbPlan.WeekStartDate
			mealPlans = append(mealPlans, plan)
		}
	}
	return mealPlans, nil
}

// ExistsForWeek checks if a plan already exists for a user on a given week.
func (r *PlanRepository) ExistsForWeek(ctx context.Context, userID string, weekStart time.Time) (bool, error) {
	count, err := r.queries.CheckPlanExists(ctx, db.CheckPlanExistsParams{
		UserID:        userID,
		WeekStartDate: weekStart,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
