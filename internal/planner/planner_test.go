package planner

import (
	"context"
	"os"
	"strings"
	"testing"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

type MockEmbedingGenerator struct{}

func (m *MockEmbedingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{1.0, 0.0}, nil // Matches Pasta
}

type MockTextGenerator struct{}

func (m *MockTextGenerator) GenerateContent(ctx context.Context, prompt string) (llm.ContentResponse, error) {
	// If it's the Analyst prompt
	if strings.Contains(prompt, "# Analyst Agent Prompt") {
		return llm.ContentResponse{
			Content: `{"planned_meals": [{"day": "Monday", "action": "Cook", "recipe_title": "Pasta", "note": "Yum"}]}`,
		}, nil
	}
	// It's the Chef prompt
	return llm.ContentResponse{
		Content: `{"plan": [{"day": "Monday", "recipe_title": "Cook: Pasta", "prep_time": "15 mins", "note": "Yum"}], "shopping_list": ["Pasta", "Tomato"]}`,
	}, nil
}

func TestGeneratePlan(t *testing.T) {
	ctx := context.Background()

	// 1. Setup temporary storage
	tempDir, _ := os.MkdirTemp("", "planner_test")
	defer os.RemoveAll(tempDir)
	store, _ := storage.NewRecipeStore(tempDir)

	// 2. Add some recipes to storage
	rec1 := recipe.NormalizedRecipe{Title: "Pasta", Embedding: []float32{1.0, 0.0}, Ingredients: []string{"Pasta", "Tomato"}}
	rec2 := recipe.NormalizedRecipe{Title: "Salad", Embedding: []float32{0.0, 1.0}, Ingredients: []string{"Lettuce", "Tomato"}}
	_ = store.Save("1", "2023-01-01T00:00:00Z", rec1)
	_ = store.Save("2", "2023-01-01T00:00:00Z", rec2)

	planner := NewPlanner(store, &MockTextGenerator{}, &MockEmbedingGenerator{})

	// 4. Run GeneratePlan
	plan, metas, err := planner.GeneratePlan(ctx, "I want pasta", PlanningContext{})
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	// 5. Assertions
	if len(metas) != 2 {
		t.Errorf("Expected 2 meta entries, got %d", len(metas))
	}
	if len(plan.Plan) != 1 {
		t.Errorf("Expected 1 day in plan, got %d", len(plan.Plan))
	}
	if plan.Plan[0].RecipeTitle != "Cook: Pasta" {
		t.Errorf("Expected Monday recipe to be 'Cook: Pasta', got '%s'", plan.Plan[0].RecipeTitle)
	}
	if len(plan.ShoppingList) != 2 {
		t.Errorf("Expected 2 items in shopping list, got %d", len(plan.ShoppingList))
	}
}
