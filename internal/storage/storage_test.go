package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"ai-meal-planner/internal/recipe"
)

func TestRecipeStore(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "storage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewRecipeStore(tempDir)
	if err != nil {
		t.Fatalf("Failed to create RecipeStore: %v", err)
	}

	recipeID := "test-recipe-123"
	updatedAt := "2023-10-27T10:00:00Z"
	sanitizedUpdatedAt := "2023-10-27T10-00-00Z" // Expectation based on sanitize logic

	rec := recipe.NormalizedRecipe{
		Title:        "Test Recipe",
		Ingredients:  []string{"1 cup of testing"},
		Instructions: "Write a test.",
		Tags:         []string{"go", "test"},
	}

	t.Run("CheckExists-False", func(t *testing.T) {
		if store.Exists(recipeID, updatedAt) {
			t.Errorf("Expected recipe '%s' version '%s' to not exist, but it does", recipeID, updatedAt)
		}
	})

	t.Run("Save", func(t *testing.T) {
		if err := store.Save(recipeID, updatedAt, rec); err != nil {
			t.Fatalf("Failed to save recipe: %v", err)
		}

		// Verify file was created with sanitized name
		expectedFilename := fmt.Sprintf("%s_%s.json", recipeID, sanitizedUpdatedAt)
		filePath := filepath.Join(tempDir, expectedFilename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file '%s' to be created, but it wasn't", filePath)
		}
	})

	t.Run("CheckExists-True", func(t *testing.T) {
		if !store.Exists(recipeID, updatedAt) {
			t.Errorf("Expected recipe '%s' version '%s' to exist, but it doesn't", recipeID, updatedAt)
		}
	})

	t.Run("Load", func(t *testing.T) {
		loadedRec, err := store.Load(recipeID, updatedAt)
		if err != nil {
			t.Fatalf("Failed to load recipe: %v", err)
		}

		if loadedRec.Title != rec.Title {
			t.Errorf("Expected title '%s', got '%s'", rec.Title, loadedRec.Title)
		}
		if len(loadedRec.Ingredients) != 1 {
			t.Errorf("Expected 1 ingredient, got %d", len(loadedRec.Ingredients))
		}
	})

	t.Run("RemoveStaleVersions", func(t *testing.T) {
		// Create a few stale files
		oldVersion := "2023-01-01T00:00:00Z"
		if err := store.Save(recipeID, oldVersion, rec); err != nil {
			t.Fatalf("Failed to save old version: %v", err)
		}
		
		// Ensure both exist
		if !store.Exists(recipeID, updatedAt) || !store.Exists(recipeID, oldVersion) {
			t.Fatal("Setup failed: expected both versions to exist")
		}

		// Remove all versions for this ID
		if err := store.RemoveStaleVersions(recipeID); err != nil {
			t.Fatalf("Failed to remove stale versions: %v", err)
		}

		// Verify they are gone
		if store.Exists(recipeID, updatedAt) {
			t.Error("Expected updated version to be removed")
		}
		if store.Exists(recipeID, oldVersion) {
			t.Error("Expected old version to be removed")
		}
	})

	t.Run("FindSimilar", func(t *testing.T) {
		// Clear directory
		files, _ := filepath.Glob(filepath.Join(tempDir, "*"))
		for _, f := range files {
			os.Remove(f)
		}

		// Save 3 recipes with mocked embeddings
		// Vector A: [1, 0]
		// Vector B: [1, 0] (Identical to A)
		// Vector C: [0, 1] (Orthogonal to A)
		
		recA := recipe.NormalizedRecipe{Title: "Recipe A", Embedding: []float32{1.0, 0.0}}
		recB := recipe.NormalizedRecipe{Title: "Recipe B", Embedding: []float32{1.0, 0.0}}
		recC := recipe.NormalizedRecipe{Title: "Recipe C", Embedding: []float32{0.0, 1.0}}

		_ = store.Save("A", "2023-01-01T00:00:00Z", recA)
		_ = store.Save("B", "2023-01-01T00:00:00Z", recB)
		_ = store.Save("C", "2023-01-01T00:00:00Z", recC)

		// Search for something similar to A ([1, 0])
		results, err := store.FindSimilar([]float32{1.0, 0.0}, 2)
		if err != nil {
			t.Fatalf("FindSimilar failed: %v", err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
		
		// A and B should be top results (order might vary between equal scores, but C should not be there)
		foundA := false
		foundB := false
		for _, r := range results {
			if r.Title == "Recipe A" { foundA = true }
			if r.Title == "Recipe B" { foundB = true }
			if r.Title == "Recipe C" { t.Error("Recipe C should not be in top 2") }
		}

		if !foundA || !foundB {
			t.Error("Expected to find both Recipe A and Recipe B")
		}
	})
}