package planner

import (
	"ai-meal-planner/internal/value"
	"context"
)

type mockSearcher struct {
	recipes []value.Recipe
}

func (m *mockSearcher) GetByIds(ctx context.Context, recipeIds []string) ([]value.Recipe, error) {
	return m.recipes, nil
}

func (m *mockSearcher) RandomRecipes(ctx context.Context, limit int64, excludeIDs []string) ([]value.Recipe, error) {
	return m.recipes, nil
}

func (m *mockSearcher) RecipeSemanticSearch(ctx context.Context, query string, excludeIDs []string) ([]value.Recipe, error) {
	// Filter out excluded IDs manually to simulate real DB behavior
	var filtered []value.Recipe
	excludedMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludedMap[id] = true
	}

	for _, r := range m.recipes {
		if !excludedMap[r.ID] {
			filtered = append(filtered, r)
		}
	}

	// In a real scenario, the LLM would see these.
	// We return all non-excluded ones to see if the LLM picks the Vegan one.
	return filtered, nil
}
