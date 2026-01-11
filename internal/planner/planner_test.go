package planner

import (
	"context"
	"os"
	"testing"

	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

type mockPlannerLLM struct {
	embeddingResponse []float32
	contentResponse   string
}

func (m *mockPlannerLLM) GenerateContent(ctx context.Context, prompt string) (string, error) {
	return m.contentResponse, nil
}

func (m *mockPlannerLLM) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.embeddingResponse, nil
}

func (m *mockPlannerLLM) Close() error { return nil }

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

	// 3. Mock LLM
	mockLLM := &mockPlannerLLM{
		embeddingResponse: []float32{1.0, 0.0}, // Matches Pasta
		contentResponse: `{
			"plan": [
				{"day": "Monday", "recipe_title": "Pasta", "note": "Yum"}
			],
			"shopping_list": ["Pasta", "Tomato"],
			"total_prep_estimate": "30 mins"
		}`,
	}

	planner := NewPlanner(store, mockLLM)

	// 4. Run GeneratePlan
	plan, err := planner.GeneratePlan(ctx, "I want pasta")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	// 5. Assertions
	if len(plan.Plan) != 1 {
		t.Errorf("Expected 1 day in plan, got %d", len(plan.Plan))
	}
	if plan.Plan[0].RecipeTitle != "Pasta" {
		t.Errorf("Expected Monday recipe to be 'Pasta', got '%s'", plan.Plan[0].RecipeTitle)
	}
	if len(plan.ShoppingList) != 2 {
		t.Errorf("Expected 2 items in shopping list, got %d", len(plan.ShoppingList))
	}
}
