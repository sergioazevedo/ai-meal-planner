package planner

import (
	"context"
	"strings"
	"testing"
	"time"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
)

// Run with: go test -v ./internal/planner -run TestPlanReviewer_LiveEval
func TestPlanReviewer_LiveEval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live eval in short mode")
	}

	ctx := context.Background()
	cfg, err := config.NewFromEnv()
	if err != nil {
		t.Skip("Skipping: No API keys found in environment")
	}

	// 1. Setup the Agent and dependencies
	// Note: We use ModelAnalyst here if ModelReviewer isn't explicitly defined in groq.go, 
	// as it represents the "High Reasoning" model needed for this complex task.
	groqClient := llm.NewGroqClient(cfg, llm.ModelAnalyst, 0.1) 
	
	// Create a simple mock searcher that returns vegetarian options when called
	mockSearcher := &mockSearcher{
		recipes: []recipe.Recipe{
			{ID: "r_veg1", Title: "Vegetarian Lentil Curry", Tags: []string{"Vegetarian", "Curry"}},
			{ID: "r_veg2", Title: "Mushroom Risotto", Tags: []string{"Vegetarian", "Rice"}},
		},
	}
	reviewer := NewPlanReviewer(groqClient, mockSearcher)

	// 2. Define the Starting State (Current Plan)
	currentPlan := &MealPlan{
		WeekStart: time.Now(),
		Plan: []DayPlan{
			{Day: "Monday", RecipeID: "r1", RecipeTitle: "Chicken Roast", Note: "Cook"},
			{Day: "Tuesday", RecipeID: "r1", RecipeTitle: "Chicken Roast", Note: "Reuse Monday's meal"},
			{Day: "Wednesday", RecipeID: "r2", RecipeTitle: "Beef Stew", Note: "Cook"},
		},
	}

	// 3. Define the Feedback
	userRequest := "I want a standard week of dinners" // Original request
	feedback := "Can you make Wednesday vegetarian instead?"
	pCtx := PlanningContext{Adults: 2, Children: 0}
	recipesRecentlyUsed := []string{"r1", "r2"} // Pass the current plan's recipes

	// 4. Run the Agent
	t.Log("Executing PlanReviewer Agent...")
	result, err := reviewer.Run(ctx, currentPlan, userRequest, feedback, pCtx, recipesRecentlyUsed)
	if err != nil {
		t.Fatalf("PlanReviewer failed to respond: %v", err)
	}

	revised := result.RevisedPlan

	// 5. Quality Assertions (The "Evals")
	
	// EVAL A: Structural Integrity
	if len(revised.Plan) != 3 {
		t.Fatalf("QUALITY FAIL: Plan length changed. Expected 3, got %d", len(revised.Plan))
	}

	// EVAL B: Preservation ("Preserve Good Parts" rule)
	if revised.Plan[0].RecipeTitle != "Chicken Roast" || revised.Plan[1].RecipeTitle != "Chicken Roast" {
		t.Errorf("QUALITY FAIL: Agent modified Monday/Tuesday. Expected 'Chicken Roast', got '%s'/'%s'", 
			revised.Plan[0].RecipeTitle, revised.Plan[1].RecipeTitle)
	}

	// EVAL C: Modification Success
	wednesday := revised.Plan[2]
	if wednesday.RecipeTitle == "Beef Stew" {
		t.Errorf("QUALITY FAIL: Agent failed to change Wednesday. It is still 'Beef Stew'")
	}
	
	isVegetarian := strings.Contains(strings.ToLower(wednesday.RecipeTitle), "vegetarian") || 
					strings.Contains(strings.ToLower(wednesday.RecipeTitle), "mushroom")
					
	if !isVegetarian {
		t.Errorf("QUALITY FAIL: Agent changed Wednesday, but not to a recognized vegetarian option. Got: '%s'", wednesday.RecipeTitle)
	}

	// EVAL D: Tool Usage
	if len(result.Meta.ToolCalls) == 0 {
		t.Errorf("QUALITY FAIL: Agent did not use the 'search_recipes' tool to find a replacement.")
	} else {
		t.Logf("Agent successfully used tool '%s' with input: %v", 
			result.Meta.ToolCalls[0].ToolName, result.Meta.ToolCalls[0].Input)
	}

	t.Logf("✅ Eval complete. Revised plan successfully updated Wednesday to: %s", wednesday.RecipeTitle)
}

// mockSearcher is a lightweight helper for the eval test
type mockSearcher struct {
	recipes []recipe.Recipe
}

func (m *mockSearcher) GetRecipeCandidates(ctx context.Context, query string, excludeIDs []string) ([]recipe.Recipe, error) {
	// Simple mock: just return the static list regardless of the exact query
	return m.recipes, nil
}
