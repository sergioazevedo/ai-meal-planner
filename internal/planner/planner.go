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
	// We fetch 15 recipes to give the LLM enough variety to choose from
	recipes, err := p.recipeStore.FindSimilar(queryEmbedding, 15)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	if len(recipes) == 0 {
		return nil, fmt.Errorf("no recipes found to create a plan")
	}

	// 3. Construct the prompt with recipes as context
	var contextBuilder strings.Builder
	for i, r := range recipes {
		fmt.Fprintf(&contextBuilder, "Recipe %d:\nTitle: %s\nTags: %v\nIngredients: %v\nPrep Time: %s\nServings: %s\n\n",
			i+1, r.Title, r.Tags, r.Ingredients, r.PrepTime, r.Servings)
	}

	prompt := fmt.Sprintf(`
You are an expert meal planner. Based on the user's request and the provided list of recipes, create a 7-day meal plan.

User Request: "%s"

Household Composition:
- Adults: %d
- Children: %d (Ages: %v)

Cooking Constraints:
- Target cooking frequency: %d times per week. 
- Strategy: Typically 3 times during weekdays and 1-2 times on weekends.
- On non-cooking days, the plan MUST utilize leftovers.

Available Recipes:
%s

Instructions:
1. **Portion Scaling & Batch Cooking**: 
   - Calculate total portions needed per meal: Adult = 1.0, Child (0-10) = 0.5.
   - When cooking, cook enough portions to cover the subsequent "Leftover" days. 
   - Scale the "Ingredients" in the shopping list based on the total portions required for the whole week.
2. **Leftover Optimization**:
   - Prioritize recipes that are "Better the next day" (stews, curries, lasagnas, bakes).
   - Explicitly mark days as "Cook: [Recipe Name]" or "Leftovers: [Recipe Name]".
3. **Language Detection**: Analyze the language of the "User Request".
4. **Response Language**: Generate 'note', 'prep_time', and 'shopping_list' in the same language as the User Request. 'recipe_title' stays in its original language.
5. **Return Format**: Strictly JSON:
{
  "plan": [
    {
      "day": "Monday", 
      "recipe_title": "Cook: [Name]", 
      "prep_time": "45 mins",
      "note": "Cooking 2.5 portions to cover today and tomorrow."
    },
    {
      "day": "Tuesday", 
      "recipe_title": "Leftovers: [Name]", 
      "prep_time": "5 mins",
      "note": "Enjoying leftovers from Monday."
    }
  ],
  "shopping_list": ["item 1 (scaled for total week portions)", ...],
  "total_prep_estimate": "Total time"
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
