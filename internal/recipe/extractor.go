package recipe

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"bytes"
	"context"
	"crypto/md5"
	_ "embed"
	"encoding/hex"
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
	vectorRepo *llm.VectorRepository, // Add vectorRepo here for checking existing embeddings
	data PostData,
) (RecipeWithEmbedding, shared.AgentMeta, error) {
	result, err := runExtractor(ctx, textGen, data)
	if err != nil {
		return RecipeWithEmbedding{}, shared.AgentMeta{}, err
	}

	embeddingSourceText := result.Recipe.ToEmbeddingText()
	hasher := md5.New()
	hasher.Write([]byte(embeddingSourceText))
	currentTextHash := hex.EncodeToString(hasher.Sum(nil))

	var embedding []float32
	var meta = result.Meta

	// Try to retrieve existing embedding and hash
	existingEmbeddingRecord, err := vectorRepo.Get(ctx, result.Recipe.ID)
	if err != nil && err != sql.ErrNoRows { // Handle real errors, ignore no rows
		return RecipeWithEmbedding{}, result.Meta, fmt.Errorf("failed to get existing embedding record: %w", err)
	}

	if existingEmbeddingRecord != nil && existingEmbeddingRecord.TextHash == currentTextHash {
		// Cache HIT: use existing embedding
		embedding = existingEmbeddingRecord.Embedding
		meta.Usage.PromptTokens = 0 // No tokens consumed for embedding
		meta.Usage.CompletionTokens = 0
		meta.Latency = 0 // No latency for embedding generation
	} else {
		// Cache MISS or hash mismatch: generate new embedding
		embedding, err = embGen.GenerateEmbedding(ctx, embeddingSourceText)
		if err != nil {
			return RecipeWithEmbedding{}, result.Meta, fmt.Errorf("failed to generate embedding: %w", err)
		}
	}

	// Save the embedding (will upsert in DB) with the new hash
	// This ensures the hash is always up-to-date even if only recipe data changed.
	if err := vectorRepo.Save(ctx, result.Recipe.ID, embedding, currentTextHash); err != nil {
		return RecipeWithEmbedding{}, result.Meta, fmt.Errorf("failed to save embedding with hash: %w", err)
	}

	return RecipeWithEmbedding{
		Recipe:    result.Recipe,
		Embedding: embedding,
	}, meta, nil
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
