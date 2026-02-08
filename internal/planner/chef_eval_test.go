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

// TestChef_LiveEval performs a real LLM call to evaluate the Chef's
// ability to format the plan and consolidate the shopping list.
// Run with: go test -v ./internal/planner -run TestChef_LiveEval
func TestChef_LiveEval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live eval in short mode")
	}

	ctx := context.Background()
	cfg, err := config.NewFromEnv()
	if err != nil {
		t.Skip("Skipping: No API keys found in environment")
	}

	groqClient := llm.NewGroqClient(cfg, llm.ModelGroqLlama31_8B)
	p := &Planner{chefGenerator: groqClient}

	// 1. Setup a Mock Proposal (The input the Chef expects from the Analyst)
	proposal := &MealProposal{
		PlannedMeals: []PlannedMeal{
			{Day: "Monday", Action: MealActionCook, RecipeTitle: "Garlic Pasta", Note: "Easy start"},
			{Day: "Tuesday", Action: MealActionLeftOvers, RecipeTitle: "Garlic Pasta", Note: "Reheat"},
		},
		Recipes: []recipe.Recipe{
			{
				Title:       "Garlic Pasta",
				PrepTime:    "20 mins",
				Servings:    "2 people",
				Ingredients: []string{"200g Pasta", "2 cloves Garlic", "Olive Oil"},
			},
		},
		Adults:   2,
		Children: 0,
	}

	// 2. Execute
	result, err := p.runChef(ctx, proposal, time.Now())
	if err != nil {
		t.Fatalf("Chef failed to respond: %v", err)
	}
	plan := result.Plan

	// 3. Quality Assertions (The "Evals")

	// EVAL A: Formatting Rule
	// Monday was "Cook", so it should be prefixed
	if !strings.HasPrefix(plan.Plan[0].RecipeTitle, "Cook:") {
		t.Errorf("FORMAT FAIL: Monday recipe title '%s' missing 'Cook:' prefix", plan.Plan[0].RecipeTitle)
	}
	// Tuesday was "Reuse", so it should be prefixed "Leftovers:"
	if !strings.HasPrefix(plan.Plan[1].RecipeTitle, "Leftovers:") {
		t.Errorf("FORMAT FAIL: Tuesday recipe title '%s' missing 'Leftovers:' prefix", plan.Plan[1].RecipeTitle)
	}

	// EVAL B: Prep Time logic
	// Leftovers should have a very short prep time (5-10 mins) regardless of the original recipe
	t.Logf("Leftover prep time: %s", plan.Plan[1].PrepTime)
	if strings.Contains(plan.Plan[1].PrepTime, "20") {
		t.Errorf("LOGIC FAIL: Leftovers should not take 20 mins to prepare.")
	}

	// EVAL C: Shopping List Aggregation & Quantities
	// We only have one recipe, so the ingredients should be present.
	foundGarlic := false
	hasQuantities := true
	for _, item := range plan.ShoppingList {
		if strings.Contains(strings.ToLower(item), "garlic") {
			foundGarlic = true
		}
		// Basic check: item should probably contain a digit if it has a quantity (like 200g or 2 cloves)
		// Or at least not be just the name. This is heuristic.
		if !containsDigit(item) && !strings.Contains(strings.ToLower(item), "olive oil") {
			hasQuantities = false
		}
	}
	if !foundGarlic {
		t.Errorf("DATA FAIL: Garlic missing from shopping list.")
	}
	if !hasQuantities {
		t.Errorf("DATA FAIL: Shopping list items missing quantities.")
	}

	t.Logf("✅ Eval complete. Chef generated a plan with %d days and %d shopping items.",
		len(plan.Plan), len(plan.ShoppingList))
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}
