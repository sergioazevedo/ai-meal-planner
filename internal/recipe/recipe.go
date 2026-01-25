package recipe

import (
	"context"
	"encoding/json"
	"fmt"

	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
)

// NormalizedRecipe represents a recipe after being normalized by the LLM.
type NormalizedRecipe struct {
	Title           string    `json:"title"`
	Ingredients     []string  `json:"ingredients"`
	Instructions    string    `json:"instructions"`
	Tags            []string  `json:"tags"`
	PrepTime        string    `json:"prep_time"`
	Servings        string    `json:"servings"`
	SourceUpdatedAt string    `json:"source_updated_at"`
	Embedding       []float32 `json:"embedding"`
}

// NormalizeRecipeHTML takes HTML content and uses an LLM to normalize it into a structured Recipe.
func NormalizeRecipeHTML(ctx context.Context, textGen llm.TextGenerator, embedGen llm.EmbeddingGenerator, post ghost.Post) (*NormalizedRecipe, error) {
	prompt := fmt.Sprintf(`
	You are a helpful assistant that extracts structured recipe information from HTML content.
	Please extract the recipe title, ingredients (including quantities), step-by-step instructions, and relevant tags from the following HTML.
	Also, extract or estimate the preparation time (e.g., "30 mins") and the number of servings (e.g., "4 people").
	
	Return the output as a JSON object with the following structure:
	{
		"title": "Recipe Name",
		"ingredients": ["quantity + name", "quantity + name", ...],
		"instructions": "Step-by-step instructions",
		"tags": ["tag1", "tag2"],
		"prep_time": "Estimated time",
		"servings": "Estimated servings"
	}
	
	Ensure the output is valid JSON. Do not include any other text in your response.
	Return ONLY the raw JSON string. Do not wrap the response in markdown code blocks.

	HTML Content for "%s":
	%s
	`, post.Title, post.HTML)

	resp, err := textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM response: %w", err)
	}

	var normalizedRecipe NormalizedRecipe
	if err := json.Unmarshal([]byte(resp.Content), &normalizedRecipe); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LLM response into NormalizedRecipe: %w. LLM Response: %s", err, resp.Content)
	}

	// Generate Embedding
	// We create a semantic string representation of the recipe for the embedding model.
	embeddingText := fmt.Sprintf("Title: %s\nTags: %v\nIngredients: %v\nPrep Time: %s",
		normalizedRecipe.Title, normalizedRecipe.Tags, normalizedRecipe.Ingredients, normalizedRecipe.PrepTime)

	embedding, err := embedGen.GenerateEmbedding(ctx, embeddingText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	normalizedRecipe.Embedding = embedding

	return &normalizedRecipe, nil
}
