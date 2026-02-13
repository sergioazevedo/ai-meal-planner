package acceptance_tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"

	_ "modernc.org/sqlite"
)

// --- Mock Ghost Client ---
type mockGhostClient struct {
	fetchRecipesCalls int
}

func (m *mockGhostClient) FetchRecipes() ([]ghost.Post, error) {
	m.fetchRecipesCalls++
	return []ghost.Post{
		{ID: "1", Title: "Test Recipe", HTML: "<h1>Test</h1>", UpdatedAt: "2023-10-27T10:00:00Z"},
	}, nil
}

func (m *mockGhostClient) CreatePost(title, html string, tags []string, publish bool) (*ghost.Post, error) {
	return &ghost.Post{ID: "new-id", Title: title, HTML: html}, nil
}

// --- Mock LLM Client ---
type mockLLMClient struct {
}

type MockTextGenerator struct {
	generateContentCalls int
}

func (m *MockTextGenerator) GenerateContent(ctx context.Context, prompt string) (llm.ContentResponse, error) {
	m.generateContentCalls++
	if strings.Contains(prompt, "ingredients\": [\"quantity + name") {
		return llm.ContentResponse{
			Content: `{
				"title": "Test Recipe",
				"ingredients": ["1 cup testing"],
				"instructions": ["Write a test."],
				"tags": ["go", "test"],
				"prep_time": "10 mins",
				"servings": "1"
			}`,
		}, nil
	}

	if strings.Contains(prompt, "Strategic Meal Planning Analyst") {
		return llm.ContentResponse{
			Content: `{
				"planned_meals": [
					{"day": "Monday", "action": "Cook", "recipe_title": "Test Recipe", "note": "Only one available"}
				]
			}`,
		}, nil
	}

	return llm.ContentResponse{
		Content: `{
			"plan": [
				{"day": "Monday", "recipe_title": "Cook: Test Recipe", "prep_time": "10 mins", "note": "Only one available"}
			],
			"shopping_list": ["1 cup testing"]
		}`,
	}, nil
}

type MockEmbedingGenerator struct {
	shouldError bool
}

func (m *MockEmbedingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.0, 0.0}, nil
}

// --- Acceptance Test ---
func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	// 1. Set up a temporary database file
	tempFile, err := os.CreateTemp("", "acceptance_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(dbPath)

	// 2. Initialize mocks and real repositories
	ghostClient := &mockGhostClient{}
	mockTextGenerator := &MockTextGenerator{}
	mockEmbedingGenerator := &MockEmbedingGenerator{}

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	planRepo := planner.NewPlanRepository(db.SQL)

	metricsStore := metrics.NewStore(db.SQL)

	// 3. Create the application instance with mocks
	mealPlanner := planner.NewPlanner(recipeRepo, vectorRepo, planRepo, mockTextGenerator, mockTextGenerator, mockEmbedingGenerator)
	recipeClipper := clipper.NewClipper(ghostClient, mockTextGenerator)
	application := app.NewApp(ghostClient, mockTextGenerator, mockEmbedingGenerator, metricsStore, mealPlanner, recipeClipper, &config.Config{
		DefaultAdults:           2,
		DefaultCookingFrequency: 7,
	}, db, recipeRepo, vectorRepo, planRepo)

	// --- 4. Step 1: Ingestion ---
	t.Log("--- Step 1: Ingesting Recipes ---")
	if err := application.IngestRecipes(ctx, false); err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	if mockTextGenerator.generateContentCalls != 1 {
		t.Errorf("Expected 1 call to LLM for normalization, got %d", mockTextGenerator.generateContentCalls)
	}

	// Verify the recipe is in the database
	rec, err := recipeRepo.Get(ctx, "1")
	if err != nil {
		t.Fatalf("Failed to get recipe from DB: %v", err)
	}
	if rec.Title == "" {
		t.Errorf("Expected recipe to be in DB")
	}

	// --- 5. Step 2: Planning ---
	t.Log("--- Step 2: Generating Meal Plan ---")
	// Reset counter for planning step
	mockTextGenerator.generateContentCalls = 0

	if err := application.GenerateMealPlan(ctx, "test_user", "Give me something simple"); err != nil {

		t.Fatalf("Meal planning failed: %v", err)

	}

	if mockTextGenerator.generateContentCalls != 2 {

		t.Errorf("Expected 2 calls to LLM for planning (Analyst + Chef), got %d", mockTextGenerator.generateContentCalls)

	}

}
