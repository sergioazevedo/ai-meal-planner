package planner

import (
	"context"
	"testing"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
)

// TestAnalyst_LiveEval performs a real LLM call to evaluate the Analyst's
// strategic reasoning and rule adherence.
// Run with: go test -v ./internal/planner -run TestAnalyst_LiveEval
func TestAnalyst_LiveEval(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping live eval in short mode")
	}

	// 1. Setup real environment
	ctx := context.Background()
	cfg, err := config.NewFromEnv()
	if err != nil {
		t.Skip("Skipping: No API keys found in environment")
	}

	// Use Groq for fast, cheap evals
	groqClient := llm.NewGroqClient(cfg, llm.ModelAnalyst, 0.1)
	p := &Planner{analystGenerator: groqClient}

	// 2. Define a "Hard" Scenario
	userRequest := "We need high-protein meals for the week, but my kids hate spicy food."

	// Provide a mix of spicy and non-spicy recipes
	mockRecipes := []recipe.Recipe{
		{Title: "Spicy Chili Con Carne", Tags: []string{"Spicy", "Beef", "High-Protein"}},
		{Title: "Mild Chicken Thighs", Tags: []string{"Kid-Friendly", "Chicken", "High-Protein"}},
		{Title: "Lentil Soup", Tags: []string{"Vegetarian", "Healthy"}},
		{Title: "Beef Stew", Tags: []string{"Beef", "High-Protein", "Slow-Cook"}},
		{Title: "Salmon & Asparagus", Tags: []string{"Fish", "Quick", "Light"}},
		{Title: "Tofu Stir Fry", Tags: []string{"Vegan", "Tofu"}},
		{Title: "Turkey Meatballs", Tags: []string{"Kid-Friendly", "Turkey", "High-Protein"}},
		{Title: "Greek Salad", Tags: []string{"Fresh", "Vegetarian", "Light"}},
		{Title: "Pasta Bolognese", Tags: []string{"Pasta", "Beef", "Family"}},
	}

	pCtx := PlanningContext{
		Adults:       2,
		Children:     2,
		ChildrenAges: []int{5, 8},
	}

	// 3. Execute
	result, err := p.runAnalyst(ctx, userRequest, pCtx, mockRecipes)
	if err != nil {
		t.Fatalf("Analyst failed to respond: %v", err)
	}
	proposal := result.Proposal

	// 4. Quality Assertions (The "Evals")

	// EVAL A: Did it respect the "No Spicy" constraint?
	for _, meal := range proposal.PlannedMeals {
		if meal.RecipeTitle == "Spicy Chili Con Carne" {
			t.Errorf("QUALITY FAIL: Analyst picked a 'Spicy' recipe despite user constraint.")
		}
	}

	// EVAL B: Did it follow the 5-session batch-cooking strategy?
	// Monday (0) Cook, Tuesday (1) Reuse
	// Wednesday (2) Cook, Thursday (3) Reuse
	// Friday (4) Cook, Saturday Lunch (5) Reuse
	// Saturday Dinner (6) Cook, Sunday Lunch (7) Reuse
	// Sunday Dinner (8) Cook
	cadence := []MealAction{
		MealActionCook, MealActionLeftOvers,
		MealActionCook, MealActionLeftOvers,
		MealActionCook, MealActionLeftOvers,
		MealActionCook, MealActionLeftOvers,
		MealActionCook,
	}

	for i, action := range cadence {
		if proposal.PlannedMeals[i].Action != action {
			t.Errorf("STRATEGY FAIL: Slot %d (%s) should be %s, got %s",
				i, proposal.PlannedMeals[i].Day, action, proposal.PlannedMeals[i].Action)
		}
	}

	// EVAL C: Recipe consistency in bridges
	// Mon -> Tue
	if proposal.PlannedMeals[1].RecipeTitle != proposal.PlannedMeals[0].RecipeTitle {
		t.Errorf("STRATEGY FAIL: Tuesday reuse does not match Monday cook.")
	}
	// Fri -> Sat Lunch
	if proposal.PlannedMeals[5].RecipeTitle != proposal.PlannedMeals[4].RecipeTitle {
		t.Errorf("STRATEGY FAIL: Saturday Lunch reuse does not match Friday Dinner cook.")
	}
	// Sat Dinner -> Sun Lunch
	if proposal.PlannedMeals[7].RecipeTitle != proposal.PlannedMeals[6].RecipeTitle {
		t.Errorf("STRATEGY FAIL: Sunday Lunch reuse does not match Saturday Dinner cook.")
	}

	// EVAL D: Sunday Dinner should be Light
	// Sunday Dinner is usually index 8 in a 9-meal plan (Mon-Fri dinner + Sat/Sun lunch/dinner)
	sundayDinner := proposal.PlannedMeals[8]
	t.Logf("Checking Sunday Dinner: %s", sundayDinner.RecipeTitle)

	// Helper to check if the chosen recipe is one of the "Light" ones
	isLight := false
	lightRecipes := []string{"Salmon & Asparagus", "Greek Salad", "Lentil Soup"}
	for _, title := range lightRecipes {
		if sundayDinner.RecipeTitle == title {
			isLight = true
			break
		}
	}
	if !isLight {
		t.Errorf("STRATEGY FAIL: Sunday Dinner (%s) is not from the 'Light' recipe pool.", sundayDinner.RecipeTitle)
	}

	t.Logf("✅ Eval complete. Analyst proposed %d meals using %d unique recipes.",
		len(proposal.PlannedMeals), len(proposal.Recipes))
}
