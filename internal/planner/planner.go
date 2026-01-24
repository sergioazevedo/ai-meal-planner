package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/storage"
)

// DayPlan represents the plan for a single day.
type DayPlan struct {
	Day         string `json:"day"`
	RecipeTitle string `json:"recipe_title"`
	PrepTime    string `json:"prep_time"`
	Note        string `json:"note"`
}

// MealPlan represents a full weekly meal plan.
type MealPlan struct {
	Plan         []DayPlan `json:"plan"`
	ShoppingList []string  `json:"shopping_list"`
	TotalPrep    string    `json:"total_prep_estimate"`
}

// Planner handles the generation of meal plans.
type Planner struct {
	recipeStore *storage.RecipeStore
	textGen     llm.TextGenerator
	embedGen    llm.EmbeddingGenerator
}

// NewPlanner creates a new Planner instance.
func NewPlanner(store *storage.RecipeStore, textGen llm.TextGenerator, embedGen llm.EmbeddingGenerator) *Planner {
	return &Planner{
		recipeStore: store,
		textGen:     textGen,
		embedGen:    embedGen,
	}
}

// PlanningContext holds user-specific constraints for the meal plan.
type PlanningContext struct {
	Adults          int
	Children        int
	ChildrenAges    []int
	CookingFrequency int // Times per week they want to cook
}

// GeneratePlan creates a meal plan based on a user request.
func (p *Planner) GeneratePlan(ctx context.Context, userRequest string, pCtx PlanningContext) (*MealPlan, error) {
	// 1. Generate embedding for the user request to find relevant recipes
	queryEmbedding, err := p.embedGen.GenerateEmbedding(ctx, userRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for request: %w", err)
	}

	// 2. Retrieve top N relevant recipes
	// We fetch 8 recipes to give the LLM variety while staying within token limits
	recipes, err := p.recipeStore.FindSimilar(queryEmbedding, 8)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipes found to create a plan")
	}

	// 3. Construct the prompt with recipes as context
	var contextBuilder strings.Builder
	for i, r := range recipes {
		fmt.Fprintf(&contextBuilder, "R%d: %s | Tags: %v | Ingr: %v | Time: %s | Serv: %s\n",
			i+1, r.Title, r.Tags, r.Ingredients, r.PrepTime, r.Servings)
	}

	prompt := fmt.Sprintf(`
You are a highly structured meal planning assistant. Your goal is to generate a valid JSON meal plan based on the user's request and available recipes.

### Context
User Request: "%s"

Household Composition:
- Adults: %d
- Children: %d (Ages: %v)

Weekly Schedule:
- Monday to Friday: Dinner only (5 meals).
- Saturday and Sunday: Lunch AND Dinner (4 meals).
- Total meals to plan: 9.

Cooking Constraints:
- Target cooking frequency: %d times per week. 
- Weekday Strategy: Prefer 3 cooking sessions (e.g., Monday, Wednesday, Friday). Avoid cooking on consecutive weekdays.
- Weekend Strategy: Typically 2 cooking sessions (e.g., Saturday Dinner and Sunday Dinner). 
- On non-cooking days, the plan MUST utilize leftovers.

Available Recipes:
%s

### Rules
1. **Portion Scaling & Batch Cooking**: 
   - Adult = 1.0, Child (0-10) = 0.5.
   - **MANDATORY**: On Weekdays, use a "Cook for 2 days" strategy (e.g., Cook Monday to cover Tuesday).
2. **Leftover & Weekend Strategy**:
   - **DO NOT** cook on consecutive weekdays (e.g., if you cook Monday, Tuesday MUST be leftovers).
   - **Saturday Dinner** leftovers SHOULD be used for **Sunday Lunch**.
   - **Sunday Dinner** MUST be a **Light Meal**.
   - If cooking frequency < 9, select recipes that store well.
   - Mark days clearly as "Cook: [Recipe Name]" or "Leftovers: [Recipe Name]".
3. **Output Format**: 
   - Return ONLY a valid JSON object. 
   - The "plan" array must contain 9 entries starting from Monday.
4. **Language**: 
   - Use the same language as the User Request for 'note', 'prep_time', and 'shopping_list'.
   - 'recipe_title' must match the original title from the context.

### JSON Structure Example
{
  "plan": [
    { "day": "Monday", "recipe_title": "Cook: [Name]", "prep_time": "45 mins", "note": "Notes..." },
    ...
    { "day": "Saturday (Lunch)", "recipe_title": "Cook: [Name]", "prep_time": "30 mins", "note": "Notes..." },
    { "day": "Saturday (Dinner)", "recipe_title": "Cook: [Name]", "prep_time": "40 mins", "note": "Notes..." },
    { "day": "Sunday (Lunch)", "recipe_title": "Leftovers: [Name]", "prep_time": "5 mins", "note": "From Sat Dinner." },
    { "day": "Sunday (Dinner)", "recipe_title": "Cook: [Name]", "prep_time": "15 mins", "note": "Light meal." }
  ],
  "shopping_list": ["Item (quantity)", ...],
  "total_prep_estimate": "Estimated total time"
}
`, userRequest, pCtx.Adults, pCtx.Children, pCtx.ChildrenAges, pCtx.CookingFrequency, contextBuilder.String())

	// 4. Call Gemini to generate the plan
	llmResponse, err := p.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate meal plan from LLM: %w", err)
	}

	// 5. Parse the response
	var mealPlan MealPlan
	if err := json.Unmarshal([]byte(llmResponse), &mealPlan); err != nil {
		return nil, fmt.Errorf("failed to parse meal plan JSON: %w. Response: %s", err, llmResponse)
	}

	return &mealPlan, nil
}
