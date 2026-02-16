package shopping

import "time"

// ShoppingList represents a shopping list for a meal plan.
type ShoppingList struct {
	ID          int64     `json:"id"`
	UserID      string    `json:"user_id"`
	MealPlanID  int64     `json:"meal_plan_id"`
	Items       []string  `json:"items"`
	CreatedAt   time.Time `json:"created_at"`
}
