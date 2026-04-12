package recipe

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/value"
)

// SearchService handles operations related to recipes, including searching and retrieving.
type SearchService struct {
	recipeRepo *Repository
	vectorRepo *llm.VectorRepository
	embedGen   llm.EmbeddingGenerator
}

// NewSearchService creates a new RecipeService instance.
func NewSearchService(
	recipeRepo *Repository,
	vectorRepo *llm.VectorRepository,
	embedGen llm.EmbeddingGenerator,
) *SearchService {
	return &SearchService{
		recipeRepo: recipeRepo,
		vectorRepo: vectorRepo,
		embedGen:   embedGen,
	}
}

const SEARCH_LIMIT = 10

// RecipeSemanticSearch retrieves recipe candidates based on a query string using semantic search.
func (s *SearchService) RecipeSemanticSearch(
	ctx context.Context,
	query string,
	excludeIDs []string,
) ([]value.Recipe, error) {
	queryEmbedding, err := s.embedGen.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for request: %w", err)
	}

	recipeIds, err := s.vectorRepo.FindSimilar(ctx, queryEmbedding, SEARCH_LIMIT, excludeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	recipes, err := s.recipeRepo.GetByIds(ctx, recipeIds)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve recipes: %w", err)
	}

	return recipes, nil
}

func (s *SearchService) RandomRecipes(
	ctx context.Context,
	limit int64,
	excludeIDs []string,
) ([]value.Recipe, error) {
	return s.recipeRepo.GetRandomReipes(ctx, SEARCH_LIMIT, excludeIDs)
}

func (s *SearchService) GetByIds(
	ctx context.Context,
	IDs []string,
) ([]value.Recipe, error) {
	return s.recipeRepo.GetByIds(ctx, IDs)
}
