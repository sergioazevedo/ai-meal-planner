package app

import (
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/value"
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ProcessAndSaveRecipe encapsulates the common logic for processing a recipe post,
// generating/caching its embedding, and saving it to the database.
func ProcessAndSaveRecipe(
	ctx context.Context,
	extractor *recipe.Extractor,
	recipeRepo *recipe.Repository,
	metricsStore *metrics.Store,
	post ghost.Post,
	force bool,
) error {
	rec, err := ensureRecipe(ctx, extractor, recipeRepo, metricsStore, post, force)
	if err != nil {
		return err
	}

	// Generate and Save Embedding
	_, meta, err := extractor.ProcessAndSaveEmbedding(ctx, rec)
	if err != nil {
		return fmt.Errorf("failed to process and save embedding: %w", err)
	}

	return metricsStore.RecordMeta(meta)
}

// ensureRecipe retrieves a recipe from the repository or extracts it from the post if missing (or if forced).
func ensureRecipe(
	ctx context.Context,
	extractor *recipe.Extractor,
	recipeRepo *recipe.Repository,
	metricsStore *metrics.Store,
	post ghost.Post,
	force bool,
) (value.Recipe, error) {
	if !force {
		rec, err := recipeRepo.Get(ctx, post.ID)
		if err == nil {
			return rec, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return value.Recipe{}, fmt.Errorf("failed to get recipe from repo: %w", err)
		}
	}

	// Extraction required
	var tags []string
	for _, t := range post.Tags {
		tags = append(tags, t.Name)
	}

	res, err := extractor.ExtractRecipe(ctx, recipe.PostData{
		ID:        post.ID,
		Title:     post.Title,
		UpdatedAt: post.UpdatedAt,
		HTML:      post.HTML,
		Tags:      tags,
	})
	if err != nil {
		return value.Recipe{}, fmt.Errorf("failed to extract recipe: %w", err)
	}

	if err := recipeRepo.Save(ctx, res.Recipe); err != nil {
		return value.Recipe{}, fmt.Errorf("failed to save recipe: %w", err)
	}

	if err := metricsStore.RecordMeta(res.Meta); err != nil {
		return res.Recipe, fmt.Errorf("failed to record extraction metrics: %w", err)
	}

	return res.Recipe, nil
}
