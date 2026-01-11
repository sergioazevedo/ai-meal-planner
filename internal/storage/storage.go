package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// sanitizeTimestamp makes the timestamp safe for filenames.
func sanitizeTimestamp(ts string) string {
	return strings.ReplaceAll(ts, ":", "-")
}

// getVersionedPath returns the full path for a given recipe ID and version.
func (s *RecipeStore) getVersionedPath(recipeID, updatedAt string) string {
	filename := fmt.Sprintf("%s_%s.json", recipeID, sanitizeTimestamp(updatedAt))
	return filepath.Join(s.basePath, filename)
}

// Save stores a normalized recipe to a file with versioning.
func (s *RecipeStore) Save(recipeID, updatedAt string, rec recipe.NormalizedRecipe) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recipe: %w", err)
	}

	filePath := s.getVersionedPath(recipeID, updatedAt)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write recipe file: %w", err)
	}
	return nil
}

// Load retrieves a normalized recipe from a specific version file.
func (s *RecipeStore) Load(recipeID, updatedAt string) (*recipe.NormalizedRecipe, error) {
	filePath := s.getVersionedPath(recipeID, updatedAt)
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

// Exists checks if a specific version of a normalized recipe file exists.
func (s *RecipeStore) Exists(recipeID, updatedAt string) bool {
	filePath := s.getVersionedPath(recipeID, updatedAt)
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// RemoveStaleVersions removes all files associated with a recipeID.
// This should be called before saving a new version to ensure only the latest exists.
func (s *RecipeStore) RemoveStaleVersions(recipeID string) error {
	pattern := filepath.Join(s.basePath, fmt.Sprintf("%s_*.json", recipeID))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to glob stale files: %w", err)
	}

	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("failed to remove stale file %s: %w", match, err)
		}
	}
	return nil
}
