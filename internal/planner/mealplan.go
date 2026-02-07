package planner

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
	Plan         []DayPlan `json:"plan"`
	ShoppingList []string  `json:"shopping_list"`
}
