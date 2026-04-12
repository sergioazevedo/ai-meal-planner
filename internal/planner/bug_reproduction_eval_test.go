package planner

import (
	"context"
	"testing"
	"time"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/value"
)

// Run with: go test -v ./internal/planner -run TestPlanReviewer_BugReproduction_Eval
func TestPlanReviewer_BugReproduction_Eval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live eval in short mode")
	}

	ctx := context.Background()
	cfg, err := config.NewFromEnv()
	if err != nil {
		t.Skip("Skipping: No API keys found in environment")
	}

	// 1. Setup the Agent and dependencies
	groqClient := llm.NewGroqClient(cfg, llm.ModelAnalyst, 0.1)

	// Mock searcher with specific candidates to test Context and History
	mockSearcher := &mockSearcher{
		recipes: []value.Recipe{
			{ID: "v_curry", Title: "Vegan Lentil Curry", PrepTime: "45min", Tags: []string{"Vegan", "Curry"}},
			{ID: "fast_beef", Title: "Fast Beef Stir Fry", PrepTime: "10min", Tags: []string{"Meat", "Quick"}},
			{ID: "fast_tofu", Title: "Quick Tofu Scramble", PrepTime: "10min", Tags: []string{"Vegan", "Quick"}},
		},
	}
	reviewer := NewPlanReviewer(groqClient, mockSearcher)

	// 2. Define the Starting State (Current Plan)
	currentPlan := &MealPlan{
		WeekStart: time.Now(),
		Plan: []DayPlan{
			{Day: "Monday", RecipeID: "v_bowl", RecipeTitle: "Slow Vegan Buddha Bowl", PrepTime: "60min", Note: "Cook"},
			{Day: "Tuesday", RecipeID: "v_bowl", RecipeTitle: "Slow Vegan Buddha Bowl", PrepTime: "5min", Note: "Reuse Monday's meal"},
			{Day: "Wednesday", RecipeID: "v_soup", RecipeTitle: "Vegan Miso Soup", PrepTime: "20min", Note: "Cook"},
		},
	}

	// 3. Define the Constraints
	userRequest := "I need a fully vegan week."
	feedback := "Can you make Monday much faster to prep?"
	pCtx := PlanningContext{Adults: 2, Children: 0}

	// SCENARIO: History Awareness (Avoid 'v_curry' because it was eaten last week)
	recipesRecentlyUsed := []string{"v_curry"}

	// 4. Run the Agent
	t.Log("Executing PlanReviewer Agent...")
	result, err := reviewer.Run(ctx, currentPlan, userRequest, feedback, pCtx, recipesRecentlyUsed)
	if err != nil {
		t.Fatalf("PlanReviewer failed to respond: %v", err)
	}

	revised := result.RevisedPlan

	// 5. Quality Assertions (The "Evals")

	// EVAL A: History Awareness
	// If the agent picked 'v_curry', it failed to respect the 'recipesRecentlyUsed' exclusion list.
	for _, day := range revised.Plan {
		if day.RecipeID == "v_curry" {
			t.Errorf("QUALITY FAIL (History Amnesia): Agent suggested 'Vegan Lentil Curry' but it was eaten last week (in excludeIDs).")
		}
	}

	// EVAL B: Global Context Retention
	// If the agent picked 'fast_beef', it forgot the original 'fully vegan week' constraint.
	monday := revised.Plan[0]
	if monday.RecipeID == "fast_beef" {
		t.Errorf("QUALITY FAIL (Context Loss): Agent suggested 'Fast Beef Stir Fry' for a vegan request.")
	}

	// EVAL C: Success / Correct Choice
	// The only valid choice that is Vegan + Fast + Not in history is 'fast_tofu'.
	if monday.RecipeID != "fast_tofu" {
		t.Errorf("QUALITY FAIL: Agent failed to find the optimal replacement ('fast_tofu'). Got: '%s' (ID: %s)", monday.RecipeTitle, monday.RecipeID)
	}

	// EVAL D: Targeted Adjustment
	// Wednesday should NOT have changed.
	if revised.Plan[2].RecipeID != "v_soup" {
		t.Errorf("QUALITY FAIL: Agent modified Wednesday unnecessarily. Expected 'v_soup', got '%s'", revised.Plan[2].RecipeID)
	}

	t.Logf("✅ Bug Reproduction Eval complete.")
	t.Logf("Monday was: %s -> Now: %s", currentPlan.Plan[0].RecipeTitle, revised.Plan[0].RecipeTitle)
}
