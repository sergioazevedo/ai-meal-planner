package storage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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
	data, err := json.Marshal(rec)
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

// ScoredRecipe holds a recipe and its similarity score.
type ScoredRecipe struct {
	Recipe recipe.NormalizedRecipe
	Score  float64
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// FindSimilar searches for recipes with embeddings similar to the query.
// It currently scans all files on disk, which is acceptable for small datasets.
func (s *RecipeStore) FindSimilar(queryEmbedding []float32, limit int) ([]recipe.NormalizedRecipe, error) {
	matches, err := filepath.Glob(filepath.Join(s.basePath, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob recipe files: %w", err)
	}

	var scoredRecipes []ScoredRecipe

	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			// Log error but continue? For now, we return error to be safe.
			return nil, fmt.Errorf("failed to read file %s: %w", match, err)
		}

		var rec recipe.NormalizedRecipe
		if err := json.Unmarshal(data, &rec); err != nil {
			return nil, fmt.Errorf("failed to unmarshal file %s: %w", match, err)
		}

		if len(rec.Embedding) == 0 {
			continue
		}

		score := cosineSimilarity(queryEmbedding, rec.Embedding)
		scoredRecipes = append(scoredRecipes, ScoredRecipe{Recipe: rec, Score: score})
	}

	// Sort by score descending
	sort.Slice(scoredRecipes, func(i, j int) bool {
		return scoredRecipes[i].Score > scoredRecipes[j].Score
	})

	// Take top K
	if limit > len(scoredRecipes) {
		limit = len(scoredRecipes)
	}

	result := make([]recipe.NormalizedRecipe, limit)
	for i := 0; i < limit; i++ {
		result[i] = scoredRecipes[i].Recipe
	}

	return result, nil
}