package recipe

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"bytes"
	"context"
	"crypto/md5"
	"database/sql" // Added for sql.ErrNoRows
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"
)

//go:embed extractor_prompt.md
var extractorPrompt string

type ExtractorResult struct {
	Recipe Recipe
	Meta   shared.AgentMeta
}

// Extractor encapsulates dependencies for recipe extraction and embedding processes.
type Extractor struct {
	textGen    llm.TextGenerator
	embGen     llm.EmbeddingGenerator
	vectorRepo llm.VectorRepositoryInterface
}

// NewExtractor creates a new Extractor instance.
func NewExtractor(textGen llm.TextGenerator, embGen llm.EmbeddingGenerator, vectorRepo llm.VectorRepositoryInterface) *Extractor {
	return &Extractor{
		textGen:    textGen,
		embGen:     embGen,
		vectorRepo: vectorRepo,
	}
}

// ExtractRecipe takes raw recipe data and extracts structured information using an LLM.
func (e *Extractor) ExtractRecipe(
	ctx context.Context,
	data PostData,
) (ExtractorResult, error) {
	start := time.Now()

	prompt, err := buildExtractorPrompt(data)
	if err != nil {
		return ExtractorResult{}, err
	}

	llmResp, err := e.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return ExtractorResult{}, fmt.Errorf("failed to get LLM response: %w", err)
	}

	rec := Recipe{}
	if err := json.Unmarshal([]byte(llmResp.Content), &rec); err != nil {
		return ExtractorResult{
				Recipe: rec,
				Meta: shared.AgentMeta{
					AgentName: "Extractor",
					Usage:     llmResp.Usage,
				},
			}, fmt.Errorf(
				"failed to get LLM response: failed to unmarshal LLM response: %w",
				err,
			)
	}

	rec.ID = data.ID
	rec.UpdatedAt = data.UpdatedAt

	// Merge manual tags from Ghost if they exist
	if len(data.Tags) > 0 {
		tagMap := make(map[string]struct{})
		for _, t := range rec.Tags {
			tagMap[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
		}
		for _, t := range data.Tags {
			tagMap[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
		}

		finalTags := make([]string, 0, len(tagMap))
		for t := range tagMap {
			finalTags = append(finalTags, t)
		}
		rec.Tags = finalTags
	}

	return ExtractorResult{
		Recipe: rec,
		Meta: shared.AgentMeta{
			AgentName: "Extractor",
			Usage:     llmResp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

// ProcessAndSaveEmbedding generates and saves the embedding for a given recipe,
// utilizing a caching mechanism.
func (e *Extractor) ProcessAndSaveEmbedding(
	ctx context.Context,
	rec Recipe, // Already extracted recipe
) (embedding []float32, meta shared.AgentMeta, err error) {
	embeddingSourceText := rec.ToEmbeddingText()
	hasher := md5.New()
	hasher.Write([]byte(embeddingSourceText))
	currentTextHash := hex.EncodeToString(hasher.Sum(nil))

	// Initialize meta for embedding generation
	embedMeta := shared.AgentMeta{AgentName: "Embedding"}

	// Try to retrieve existing embedding and hash
	existingEmbeddingRecord, err := e.vectorRepo.Get(ctx, rec.ID)
	if err != nil && err != sql.ErrNoRows {
		return nil, embedMeta, fmt.Errorf("failed to get existing embedding record: %w", err)
	}

	if existingEmbeddingRecord != nil && existingEmbeddingRecord.TextHash == currentTextHash {
		// Cache HIT: use existing embedding
		embedding = existingEmbeddingRecord.Embedding
		embedMeta.Usage.PromptTokens = 0 // No tokens consumed
		embedMeta.Usage.CompletionTokens = 0
		embedMeta.Latency = 0 // No latency
	} else {
		// Cache MISS or hash mismatch: generate new embedding
		start := time.Now()
		embedding, err = e.embGen.GenerateEmbedding(ctx, embeddingSourceText)
		if err != nil {
			return nil, embedMeta, fmt.Errorf("failed to generate embedding: %w", err)
		}
		embedMeta.Latency = time.Since(start)
		// Assume 1 token per character for simplicity for metrics, or retrieve actual usage from embGen if available
		// A more accurate metric would come from the LLM client itself if exposed.
		embedMeta.Usage.PromptTokens = len(embeddingSourceText) // Placeholder
		embedMeta.Usage.CompletionTokens = 0                    // Embeddings don't have completion tokens in this context
	}

	// Save the embedding (will upsert in DB) with the new hash
	// This ensures the hash is always up-to-date even if only recipe data changed.
	if err := e.vectorRepo.Save(ctx, rec.ID, embedding, currentTextHash); err != nil {
		return nil, embedMeta, fmt.Errorf("failed to save embedding with hash: %w", err)
	}

	return embedding, embedMeta, nil
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
