package shared

import (
	"ai-meal-planner/internal/value"
	"context"
)

// RecipeSearcher defines the interface for searching recipes.
type RecipeSearcher interface {
	RecipeSemanticSearch(ctx context.Context, query string, excludeIDs []string) ([]value.Recipe, error)
	RandomRecipes(ctx context.Context, limit int64, excludeIDs []string) ([]value.Recipe, error)
	GetByIds(ctx context.Context, recipeIDs []string) ([]value.Recipe, error)
}
