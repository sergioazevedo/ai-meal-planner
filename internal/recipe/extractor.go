package recipe

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"ai-meal-planner/internal/value"
	"bytes"
	"context"
	"crypto/md5"
	"database/sql" // Added for sql.ErrNoRows
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
	Recipe value.Recipe
	Meta   shared.AgentMeta
}

type extractionResponse struct {
	Title       string   `json:"title"`
	SideDishes  []string `json:"side_dishes"`
	Ingredients []string `json:"ingredients"`
	PrepTime    string   `json:"prep_time"`
	Servings    string   `json:"servings"`
}

// Extractor encapsulates dependencies for value.recipe extraction and embedding processes.
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

// Extractvalue.Recipe takes raw value.recipe data and extracts structured information using an LLM.
func (e *Extractor) ExtractRecipe(
	ctx context.Context,
	data PostData,
) (ExtractorResult, error) {
	start := time.Now()

	prompt, err := buildExtractorPrompt(data)
	if err != nil {
		return ExtractorResult{}, err
	}

	llmResp, err := e.textGen.GenerateContent(ctx, llm.Conversation{{Role: "user", Content: prompt}}, llm.NoTools)
	if err != nil {
		return ExtractorResult{}, fmt.Errorf("failed to get LLM response: %w", err)
	}

	var extracted extractionResponse
	cleanedJSON := llm.CleanJSON(llmResp.Message.Content)
	if err := json.Unmarshal([]byte(cleanedJSON), &extracted); err != nil {
		return ExtractorResult{
				Meta: shared.AgentMeta{
					AgentName: "Extractor",
					Usage:     llmResp.Usage,
				},
			}, fmt.Errorf(
				"failed to get LLM response: failed to unmarshal LLM response: %w",
				err,
			)
	}

	rec := value.Recipe{
		ID:          data.ID,
		Title:       extracted.Title,
		SideDishes:  extracted.SideDishes,
		Ingredients: extracted.Ingredients,
		PrepTime:    extracted.PrepTime,
		Servings:    extracted.Servings,
		UpdatedAt:   data.UpdatedAt,
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

// ProcessAndSaveEmbedding generates and saves the embedding for a given value.recipe,
// utilizing a caching mechanism.
func (e *Extractor) ProcessAndSaveEmbedding(
	ctx context.Context,
	rec value.Recipe, // Already extracted value.recipe
	force bool,
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

	embeddingMetadata := e.embGen.EmbeddingMetadata()
	cacheMatches := existingEmbeddingRecord != nil &&
		existingEmbeddingRecord.TextHash == currentTextHash &&
		existingEmbeddingRecord.Model == embeddingMetadata.Model &&
		existingEmbeddingRecord.Dimensions == embeddingMetadata.Dimensions

	if !force && cacheMatches {
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
	// This ensures the hash is always up-to-date even if only value.recipe data changed.
	if len(embedding) != embeddingMetadata.Dimensions {
		return nil, embedMeta, fmt.Errorf(
			"embedding dimensions mismatch: generator returned %d, metadata declares %d",
			len(embedding),
			embeddingMetadata.Dimensions,
		)
	}

	if err := e.vectorRepo.Save(ctx, rec.ID, embedding, currentTextHash, embeddingMetadata); err != nil {
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
