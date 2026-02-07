package recipe

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"
	"time"
)

//go:embed extractor_prompt.md
var extractorPrompt string

type ExtractorResult struct {
	Recipe Recipe
	Meta   shared.AgentMeta
}

// NormalizeHTML takes raw recipe data (usually HTML), extracts structured information
// using an LLM, and generates vector embeddings for semantic search.
func NormalizeHTML(
	ctx context.Context,
	textGen llm.TextGenerator,
	embGen llm.EmbeddingGenerator,
	data PostData,
) (RecipeWithEmbedding, shared.AgentMeta, error) {
	result, err := runExtractor(ctx, textGen, data)
	if err != nil {
		return RecipeWithEmbedding{}, shared.AgentMeta{}, err
	}

	embedding, err := embGen.GenerateEmbedding(ctx, result.Recipe.ToEmbeddingText())
	if err != nil {
		return RecipeWithEmbedding{}, result.Meta, fmt.Errorf("failed to generate embedding: %w", err)
	}

	return RecipeWithEmbedding{
		Recipe:    result.Recipe,
		Embedding: embedding,
	}, result.Meta, nil
}

func runExtractor(
	ctx context.Context,
	textGen llm.TextGenerator,
	data PostData,

) (ExtractorResult, error) {
	start := time.Now()

	prompt, err := buildExtractorPrompt(data)
	if err != nil {
		return ExtractorResult{}, err
	}

	llmResp, err := textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return ExtractorResult{}, fmt.Errorf("failed to get LLM response: %w", err)
	}

	recipe := Recipe{}
	if err := json.Unmarshal([]byte(llmResp.Content), &recipe); err != nil {
		return ExtractorResult{
				Recipe: recipe,
				Meta: shared.AgentMeta{
					AgentName: "Extractor",
					Usage:     llmResp.Usage,
				},
			}, fmt.Errorf(
				"failed to get LLM response: failed to unmarshal LLM response: %w",
				err,
			)
	}

	recipe.ID = data.ID
	recipe.UpdatedAt = data.UpdatedAt
	return ExtractorResult{
		Recipe: recipe,
		Meta: shared.AgentMeta{
			AgentName: "Extractor",
			Usage:     llmResp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func buildExtractorPrompt(data PostData) (string, error) {
	tmpl, err := template.New("normalizer").Parse(extractorPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
