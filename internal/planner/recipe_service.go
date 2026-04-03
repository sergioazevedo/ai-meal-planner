package planner

import (
	"context"
	"fmt"
	"log"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
)

// RecipeService handles operations related to recipes, including searching and retrieving.
type RecipeService struct {
	recipeRepo *recipe.Repository
	vectorRepo *llm.VectorRepository
	embedGen   llm.EmbeddingGenerator
}

// NewRecipeService creates a new RecipeService instance.
func NewRecipeService(recipeRepo *recipe.Repository, vectorRepo *llm.VectorRepository, embedGen llm.EmbeddingGenerator) *RecipeService {
	return &RecipeService{
		recipeRepo: recipeRepo,
		vectorRepo: vectorRepo,
		embedGen:   embedGen,
	}
}

// GetRecipeCandidates retrieves recipe candidates based on a query string using semantic search.
func (s *RecipeService) GetRecipeCandidates(ctx context.Context, query string, excludeIDs []string) ([]recipe.Recipe, error) {
	queryEmbedding, err := s.embedGen.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for request: %w", err)
	}

	recipeIds, err := s.vectorRepo.FindSimilar(ctx, queryEmbedding, 10, excludeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	if len(recipeIds) < 5 {
		log.Printf("Warning: Recipe pool exhausted for query '%s'. Dropping exclusions.", query)
		recipeIds, err = s.vectorRepo.FindSimilar(ctx, queryEmbedding, 10, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
		}
	}

	recipes, err := s.recipeRepo.GetByIds(ctx, recipeIds)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve recipes: %w", err)
	}

	return recipes, nil
}

// GetByIds retrieves recipes by their IDs.
func (s *RecipeService) GetByIds(ctx context.Context, ids []string) ([]recipe.Recipe, error) {
	return s.recipeRepo.GetByIds(ctx, ids)
}
