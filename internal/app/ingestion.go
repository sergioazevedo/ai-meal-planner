package app

import (
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"
	"context"
	"fmt"
)

// ProcessAndSaveRecipe encapsulates the common logic for processing a recipe post,
// generating/caching its embedding, and saving it to the database.
func ProcessAndSaveRecipe(
	ctx context.Context,
	extractor *recipe.Extractor, // Use Extractor struct
	recipeRepo *recipe.Repository,
	metricsStore *metrics.Store,
	post ghost.Post,
) error {
	// 1. Extract Recipe (Normalization)
	extractorResult, err := extractor.ExtractRecipe(
		ctx,
		recipe.PostData{
			ID:        post.ID,
			Title:     post.Title,
			UpdatedAt: post.UpdatedAt,
			HTML:      post.HTML,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to extract recipe: %w", err)
	}

	// 2. Generate and Save Embedding
	embedding, embeddingMeta, err := extractor.ProcessAndSaveEmbedding(
		ctx,
		extractorResult.Recipe,
	)
	if err != nil {
		return fmt.Errorf("failed to process and save embedding: %w", err)
	}
	_ = embedding // embedding is currently unused after saving in ProcessAndSaveEmbedding

	// 3. Save Recipe to DB
	// The recipe struct in extractorResult already contains the ID and UpdatedAt set by ExtractRecipe
	if err := recipeRepo.Save(ctx, extractorResult.Recipe); err != nil {
		return fmt.Errorf("failed to save recipe: %w", err)
	}

	// 4. Record Metrics
	// Record Extractor metrics
	metricsStore.Record(metrics.ExecutionMetric{
		AgentName:        extractorResult.Meta.AgentName,
		Model:            extractorResult.Meta.Usage.Model,
		PromptTokens:     extractorResult.Meta.Usage.PromptTokens,
		CompletionTokens: extractorResult.Meta.Usage.CompletionTokens,
		LatencyMS:        extractorResult.Meta.Latency.Milliseconds(),
	})

	// Record Embedding metrics (if tokens were actually used)
	if embeddingMeta.Usage.PromptTokens > 0 { // Only record if not a cache hit
		metricsStore.Record(metrics.ExecutionMetric{
			AgentName:        embeddingMeta.AgentName,
			Model:            embeddingMeta.Usage.Model,
			PromptTokens:     embeddingMeta.Usage.PromptTokens,
			CompletionTokens: embeddingMeta.Usage.CompletionTokens,
			LatencyMS:        embeddingMeta.Latency.Milliseconds(),
		})
	}

	return nil
}
