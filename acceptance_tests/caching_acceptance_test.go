package acceptance_tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/storage"
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

// --- Mock LLM Client ---
type mockLLMClient struct {
	generateContentCalls int
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, prompt string) (string, error) {
	m.generateContentCalls++
	// Determine if it's a normalization or a planning request based on the prompt content
	if strings.Contains(prompt, "extract structured recipe information") {
		return `{
			"title": "Test Recipe",
			"ingredients": ["1 cup testing"],
			"instructions": "Write a test.",
			"tags": ["go", "test"],
			"prep_time": "10 mins",
			"servings": "1"
		}`, nil
	}
	
	return `{
		"plan": [
			{"day": "Monday", "recipe_title": "Test Recipe", "note": "Only one available"}
		],
		"shopping_list": ["1 cup testing"],
		"total_prep_estimate": "10 mins"
	}`, nil
}

func (m *mockLLMClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.0, 0.0}, nil
}

func (m *mockLLMClient) Close() error {
	return nil
}

// --- Acceptance Test ---
func TestFullWorkflow(t *testing.T) {
	ctx := context.Background()

	// 1. Set up a temporary directory for storage
	tempDir, err := os.MkdirTemp("", "acceptance_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Initialize mocks and real store
	ghostClient := &mockGhostClient{}
	llmClient := &mockLLMClient{}
	recipeStore, err := storage.NewRecipeStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create RecipeStore: %v", err)
	}

	// 3. Create the application instance with mocks
	application := &app.App{
		GhostClient: ghostClient,
		LlmClient:   llmClient,
		RecipeStore: recipeStore,
		Planner:     planner.NewPlanner(recipeStore, llmClient),
	}

	// --- 4. Step 1: Ingestion ---
	t.Log("--- Step 1: Ingesting Recipes ---")
	if err := application.IngestRecipes(ctx); err != nil {
		t.Fatalf("Ingestion failed: %v", err)
	}

	if llmClient.generateContentCalls != 1 {
		t.Errorf("Expected 1 call to LLM for normalization, got %d", llmClient.generateContentCalls)
	}
	
	updatedAt := "2023-10-27T10:00:00Z"
	if !recipeStore.Exists("1", updatedAt) {
		t.Errorf("Expected recipe to be cached")
	}

	// --- 5. Step 2: Planning ---
	t.Log("--- Step 2: Generating Meal Plan ---")
	// Reset counter for planning step
	llmClient.generateContentCalls = 0

	if err := application.GenerateMealPlan(ctx, "Give me something simple"); err != nil {
		t.Fatalf("Meal planning failed: %v", err)
	}

	if llmClient.generateContentCalls != 1 {
		t.Errorf("Expected 1 call to LLM for planning, got %d", llmClient.generateContentCalls)
	}
}