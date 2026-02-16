package planner

import (
	db "ai-meal-planner/internal/planner/plan_db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

// Save inserts a new meal plan into the database and returns its ID.
func (r *PlanRepository) Save(ctx context.Context, userID string, planData *MealPlan) (int64, error) {
	planJSON, err := json.Marshal(planData)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal meal plan to JSON for saving: %w", err)
	}

	// Default to DRAFT status if not set
	status := string(planData.Status)
	if status == "" {
		status = string(StatusDraft)
	}

	params := db.InsertMealPlanParams{
		UserID:        userID,
		PlanData:      string(planJSON),
		WeekStartDate: planData.WeekStart,
		Status:        status,
		CreatedAt:     time.Now().UTC(),
	}

	id, err := r.queries.InsertMealPlan(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("failed to insert meal plan: %w", err)
	}

	planData.ID = id
	return id, nil
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
			plan.ID = dbPlan.ID
			plan.WeekStart = dbPlan.WeekStartDate
			plan.Status = PlanStatus(dbPlan.Status)
			mealPlans = append(mealPlans, plan)
		}
	}
	return mealPlans, nil
}

// GetByID retrieves a meal plan by its ID.
func (r *PlanRepository) GetByID(ctx context.Context, id int64) (*MealPlan, error) {
	dbPlan, err := r.queries.GetMealPlanByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get meal plan by ID: %w", err)
	}

	plan := &MealPlan{}
	if err := json.Unmarshal([]byte(dbPlan.PlanData), plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meal plan: %w", err)
	}

	plan.ID = dbPlan.ID
	plan.WeekStart = dbPlan.WeekStartDate
	plan.Status = PlanStatus(dbPlan.Status)

	return plan, nil
}

// GetDraftByUserAndWeek retrieves a draft meal plan for a user and week.
func (r *PlanRepository) GetDraftByUserAndWeek(ctx context.Context, userID string, weekStart time.Time) (*MealPlan, error) {
	dbPlan, err := r.queries.GetDraftPlanByUserAndWeek(ctx, db.GetDraftPlanByUserAndWeekParams{
		UserID:        userID,
		WeekStartDate: weekStart,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get draft plan: %w", err)
	}

	plan := &MealPlan{}
	if err := json.Unmarshal([]byte(dbPlan.PlanData), plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal meal plan: %w", err)
	}

	plan.ID = dbPlan.ID
	plan.WeekStart = dbPlan.WeekStartDate
	plan.Status = PlanStatus(dbPlan.Status)

	return plan, nil
}

// UpdateStatus updates the status of a meal plan.
func (r *PlanRepository) UpdateStatus(ctx context.Context, id int64, status PlanStatus) error {
	return r.queries.UpdatePlanStatus(ctx, db.UpdatePlanStatusParams{
		Status: string(status),
		ID:     id,
	})
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
