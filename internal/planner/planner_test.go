package planner

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	_ "modernc.org/sqlite"
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

	// 1. Setup temporary database
	tempFile, _ := os.CreateTemp("", "planner_test_*.db")
	dbPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(dbPath)

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test DB: %v", err)
	}
	defer db.Close()

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	planRepo := NewPlanRepository(db.SQL)

	// 2. Add some recipes to database
	rec1 := recipe.Recipe{ID: "1", Title: "Pasta", Ingredients: []string{"Pasta", "Tomato"}, UpdatedAt: "2023-01-01T00:00:00Z"}
	emb1 := []float32{1.0, 0.0}

	rec2 := recipe.Recipe{ID: "2", Title: "Salad", Ingredients: []string{"Lettuce", "Tomato"}, UpdatedAt: "2023-01-01T00:00:00Z"}
	emb2 := []float32{0.0, 1.0}

	_ = recipeRepo.Save(ctx, rec1)
	_ = vectorRepo.Save(ctx, rec1.ID, emb1, "dummy-hash-1")

	_ = recipeRepo.Save(ctx, rec2)
	_ = vectorRepo.Save(ctx, rec2.ID, emb2, "dummy-hash-2")

	mockGen := &MockTextGenerator{}
	p := NewPlanner(recipeRepo, vectorRepo, planRepo, mockGen, mockGen, &MockEmbedingGenerator{})

	// 4. Run GeneratePlan
	plan, metas, err := p.GeneratePlan(ctx, "test_user", "I want pasta", PlanningContext{}, time.Now())
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
