package storage

import (
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
	rec := recipe.NormalizedRecipe{
		Title:       "Test Recipe",
		Ingredients: []string{"1 cup of testing"},
		Instructions: "Write a test.",
		Tags:        []string{"go", "test"},
	}

	t.Run("CheckExists-False", func(t *testing.T) {
		if store.Exists(recipeID) {
			t.Errorf("Expected recipe '%s' to not exist, but it does", recipeID)
		}
	})

	t.Run("Save", func(t *testing.T) {
		if err := store.Save(recipeID, rec); err != nil {
			t.Fatalf("Failed to save recipe: %v", err)
		}

		// Verify file was created
		filePath := filepath.Join(tempDir, recipeID+".json")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file '%s' to be created, but it wasn't", filePath)
		}
	})

	t.Run("CheckExists-True", func(t *testing.T) {
		if !store.Exists(recipeID) {
			t.Errorf("Expected recipe '%s' to exist, but it doesn't", recipeID)
		}
	})

	t.Run("Load", func(t *testing.T) {
		loadedRec, err := store.Load(recipeID)
		if err != nil {
			t.Fatalf("Failed to load recipe: %v", err)
		}

		if loadedRec.Title != rec.Title {
			t.Errorf("Expected title '%s', got '%s'", rec.Title, loadedRec.Title)
		}
		if len(loadedRec.Ingredients) != 1 {
			t.Errorf("Expected 1 ingredient, got %d", len(loadedRec.Ingredients))
		}
		if loadedRec.Ingredients[0] != "1 cup of testing" {
			t.Errorf("Expected ingredient '1 cup of testing', got '%s'", loadedRec.Ingredients[0])
		}
	})

	t.Run("Load-NotFound", func(t *testing.T) {
		_, err := store.Load("non-existent-recipe")
		if err == nil {
			t.Fatal("Expected an error for loading non-existent recipe, got nil")
		}
	})
}
