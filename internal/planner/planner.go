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

// GeneratePlan creates a meal plan based on a user request.
func (p *Planner) GeneratePlan(ctx context.Context, userRequest string) (*MealPlan, error) {
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
Only use the recipes provided in the context below. 

User Request: "%s"

Available Recipes:
%s

Instructions:
1. Select one recipe for each of the 7 days (Monday to Sunday).
2. It's okay to repeat a recipe if it fits the user's request or if there aren't enough unique recipes.
3. Aggregate all ingredients into a consolidated shopping list.
4. Return the result strictly as a JSON object with this structure:
{
  "plan": [
    {"day": "Monday", "recipe_title": "Recipe Name", "note": "Why this was chosen"},
    ...
  ],
  "shopping_list": ["item 1", "item 2", ...],
  "total_prep_estimate": "Summary of prep time for the week"
}

Do not include any other text or formatting in your response.
`, userRequest, contextBuilder.String())

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
