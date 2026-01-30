package recipe

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"ai-meal-planner/internal/llm"
)

type PostData struct {
	ID        string
	Title     string
	UpdatedAt string
	HTML      string
}

// Recipe represents a recipe after being normalized by the LLM.
type Recipe struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions string   `json:"instructions"`
	Tags         []string `json:"tags"`
	PrepTime     string   `json:"prep_time"`
	Servings     string   `json:"servings"`
	UpdatedAt    string   `json:"source_updated_at"`
}

// returns a semantic string representation of the recipe
// used for generating embeddings and similarity search.
func (r *Recipe) ToEmbeddingText() string {
	return fmt.Sprintf(
		"Title: %s\nTags: %v\nIngredients: %v\nPrep Time: %s",
		r.Title,
		r.Tags,
		r.Ingredients,
		r.PrepTime,
	)
}

// Contains the normalized recipe and embeding
type NormalizedRecipe struct {
	Recipe
	Embedding []float32 `json:"embedding"`
}

// NormalizeHTML takes HTML content and uses an LLM to normalize it into a structured Recipe.
func NormalizeHTML(
	ctx context.Context,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	post PostData,
) (NormalizedRecipe, llm.AgentMeta, error) {
	extractorResult, err := runExtractor(ctx, textGen, post)
	if err != nil {
		return NormalizedRecipe{}, llm.AgentMeta{}, err
	}

	embedStart := time.Now()
	var embedding []float32
	embedding, err = embedGen.GenerateEmbedding(
		ctx,
		extractorResult.Recipe.ToEmbeddingText(),
	)
	if err != nil {
		return NormalizedRecipe{}, llm.AgentMeta{}, fmt.Errorf(
			"failed to generate embedding: %w",
			err,
		)
	}

	// addint the emebeding latency to the stats
	// We are ignoring the token usage count for embedings on purpose
	meta := extractorResult.Meta
	meta.Latency += time.Since(embedStart)

	return NormalizedRecipe{
		Recipe:    extractorResult.Recipe,
		Embedding: embedding,
	}, meta, nil
}
