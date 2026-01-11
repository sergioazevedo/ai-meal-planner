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
}