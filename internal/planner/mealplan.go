package planner

import "time"

// PlanStatus represents the lifecycle state of a meal plan.
type PlanStatus string

const (
	StatusDraft     PlanStatus = "DRAFT"
	StatusFinal     PlanStatus = "FINAL"
	StatusAdjusting PlanStatus = "ADJUSTING"
)

// DayPlan represents the plan for a single day.
type DayPlan struct {
	Day         string `json:"day"`
	RecipeID    string `json:"recipe_id"`
	RecipeTitle string `json:"recipe_title"`
	PrepTime    string `json:"prep_time"`
	Note        string `json:"note"`
}

// MealPlan represents a full weekly meal plan.
type MealPlan struct {
	ID              int64      `json:"id,omitempty"`     // Database ID for referencing
	WeekStart       time.Time  `json:"week_start"`
	Status          PlanStatus `json:"status"`
	Plan            []DayPlan  `json:"plan"`
	ShoppingList    []string   `json:"shopping_list,omitempty"` // Optional, only populated for FINAL plans
	OriginalRequest string     `json:"original_request,omitempty"`
}
