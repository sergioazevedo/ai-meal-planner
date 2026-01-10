package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ai-meal-planner/internal/recipe"
)

// RecipeStore provides a file-based storage for normalized recipes.
type RecipeStore struct {
	basePath string
}

// NewRecipeStore creates a new RecipeStore and ensures the base directory exists.
func NewRecipeStore(basePath string) (*RecipeStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory %s: %w", basePath, err)
	}
	return &RecipeStore{basePath: basePath}, nil
}

// getPath returns the full path for a given recipe ID.
func (s *RecipeStore) getPath(recipeID string) string {
	return filepath.Join(s.basePath, fmt.Sprintf("%s.json", recipeID))
}

// Save stores a normalized recipe to a file.
func (s *RecipeStore) Save(recipeID string, rec recipe.NormalizedRecipe) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recipe: %w", err)
	}

	filePath := s.getPath(recipeID)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write recipe file: %w", err)
	}
	return nil
}

// Load retrieves a normalized recipe from a file.
func (s *RecipeStore) Load(recipeID string) (*recipe.NormalizedRecipe, error) {
	filePath := s.getPath(recipeID)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipe file: %w", err)
	}

	var rec recipe.NormalizedRecipe
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recipe: %w", err)
	}
	return &rec, nil
}

// Exists checks if a normalized recipe file exists.
func (s *RecipeStore) Exists(recipeID string) bool {
	filePath := s.getPath(recipeID)
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}