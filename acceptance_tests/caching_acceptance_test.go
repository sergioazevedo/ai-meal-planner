package acceptance_tests

import (
	"context"
	"os"
	"testing"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/storage"
)

// --- Mock Ghost Client ---
type mockGhostClient struct {
	fetchRecipesCalls int
}

func (m *mockGhostClient) FetchRecipes() ([]ghost.Post, error) {
	m.fetchRecipesCalls++
	return []ghost.Post{
		{ID: "1", Title: "Test Recipe", HTML: "<h1>Test</h1>"},
	}, nil
}

// --- Mock LLM Client ---
type mockLLMClient struct {
	generateContentCalls int
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, prompt string) (string, error) {
	m.generateContentCalls++
	return `{
		"title": "Test Recipe",
		"ingredients": ["1 cup testing"],
		"instructions": "Write a test.",
		"tags": ["go", "test"]
	}`, nil
}

func (m *mockLLMClient) Close() error {
	return nil
}

// --- Acceptance Test ---
func TestCachingWorkflow(t *testing.T) {
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
	}

	// --- 4. First Run: Normalization and Caching ---
	t.Log("--- First Run: Normalizing and Caching ---")
	if err := application.Run(ctx); err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	// Assertions for the first run
	if ghostClient.fetchRecipesCalls != 1 {
		t.Errorf("Expected FetchRecipes to be called 1 time, got %d", ghostClient.fetchRecipesCalls)
	}
	if llmClient.generateContentCalls != 1 {
		t.Errorf("Expected GenerateContent to be called 1 time, got %d", llmClient.generateContentCalls)
	}
	if !recipeStore.Exists("1") {
		t.Errorf("Expected recipe with ID '1' to be cached, but it wasn't")
	}

	// --- 5. Second Run: Loading from Cache ---
	t.Log("--- Second Run: Loading from Cache ---")

	// Reset counters
	ghostClient.fetchRecipesCalls = 0
	llmClient.generateContentCalls = 0

	if err := application.Run(ctx); err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	// Assertions for the second run
	if ghostClient.fetchRecipesCalls != 1 {
		t.Errorf("Expected FetchRecipes to be called 1 time on second run, got %d", ghostClient.fetchRecipesCalls)
	}
	if llmClient.generateContentCalls != 0 {
		t.Errorf("Expected GenerateContent to be called 0 times on second run, got %d", llmClient.generateContentCalls)
	}
}
