package acceptance_tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/audit"
	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/llmtest"
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

func (m *mockGhostClient) FetchRecipeByID(id string) (*ghost.Post, error) {
	return &ghost.Post{ID: id, Title: "Test Recipe", HTML: "<h1>Test</h1>", UpdatedAt: "2023-10-27T10:00:00Z"}, nil
}

func (m *mockGhostClient) CreatePost(title, html string, tags []string, publish bool) (*ghost.Post, error) {
	return &ghost.Post{ID: "new-id", Title: title, HTML: html}, nil
}

// --- Mock LLM Client ---
type MockTextGenerator struct {
	generateContentCalls int
}

func (m *MockTextGenerator) GenerateContent(ctx context.Context, conversation llm.Conversation, tools []llm.Tool) (llm.ContentResponse, error) {
	m.generateContentCalls++
	var prompt string
	if len(conversation) > 0 {
		prompt = conversation[len(conversation)-1].Content
	}

	if strings.Contains(prompt, "ingredients\": [\"quantity + name") {
		return llm.ContentResponse{
			Message: llm.Message{
				Content: `{
				"title": "Test Recipe",
				"ingredients": ["1 cup testing"],
				"instructions": ["Write a test."],
				"tags": ["go", "test"],
				"prep_time": "10 mins",
				"servings": "1"
			}`,
			},
		}, nil
	}

	if strings.Contains(prompt, "Strategic Meal Planning Analyst") {
		return llm.ContentResponse{
			Message: llm.Message{
				Content: `{
				"planned_meals": [
					{"day": "Monday", "action": "Cook", "recipe_title": "Test Recipe", "note": "Only one available"}
				]
			}`,
			},
		}, nil
	}

	return llm.ContentResponse{
		Message: llm.Message{
			Content: `{
			"plan": [
				{"day": "Monday", "recipe_title": "Cook: Test Recipe", "prep_time": "10 mins", "note": "Only one available"}
			],
			"shopping_list": ["1 cup testing"]
		}`,
		},
	}, nil
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
	mockEmbeddingGenerator := &llmtest.MockEmbeddingGenerator{Values: []float32{0.0, 0.0}}

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	planRepo := planner.NewPlanRepository(db.SQL)
	auditRepo := audit.NewAuditRepository(db.SQL)

	metricsStore := metrics.NewStore(db.SQL)

	recipeService := planner.NewRecipeService(recipeRepo, vectorRepo, mockEmbeddingGenerator)
	mealPlanner := planner.NewPlanner(recipeService, planRepo, mockTextGenerator, mockTextGenerator, mockTextGenerator)
	recipeClipper := clipper.NewClipper(ghostClient, mockTextGenerator)
	application := app.NewApp(ghostClient, mockTextGenerator, mockEmbeddingGenerator, metricsStore, mealPlanner, recipeClipper, &config.Config{
		DefaultAdults:           2,
		DefaultCookingFrequency: 7,
	}, db, recipeRepo, vectorRepo, planRepo, auditRepo)

	// --- 4. Step 1: Ingestion ---
	t.Log("--- Step 1: Ingesting Recipes ---")
	if err := application.IngestRecipes(ctx, false); err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	if mockTextGenerator.generateContentCalls != 1 {
		t.Errorf("Expected 1 call to LLM for normalization, got %d", mockTextGenerator.generateContentCalls)
	}

	// Verify the recipe is in the database
	_, err = recipeRepo.Get(ctx, "1")
	if err != nil {
		t.Errorf("Expected recipe '1' in DB, got error: %v", err)
	}

	// --- 5. Step 2: Planning ---
	t.Log("--- Step 2: Generating Meal Plan ---")
	if err := application.GenerateMealPlan(ctx, "test-user", "surprise me"); err != nil {
		t.Fatalf("Planning failed: %v", err)
	}

	// Verify plan was saved
	plans, _ := planRepo.ListRecentByUserID(ctx, "test-user", 1)
	if len(plans) != 1 {
		t.Fatalf("Expected 1 plan in DB, got %d", len(plans))
	}

	// --- 6. Step 3: Shopping List ---
	t.Log("--- Step 3: Verifying Shopping List ---")
	// The planner should have generated a shopping list during the plan generation
	shoppingList, _ := application.GetShoppingListForPlan(ctx, plans[0].ID)
	if len(shoppingList) == 0 {
		t.Error("Shopping list is empty")
	}

	t.Log("Acceptance test passed successfully!")
}
