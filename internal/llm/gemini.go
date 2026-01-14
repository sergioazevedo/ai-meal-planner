package llm

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/config"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// LLMClient is an interface for a client that can interact with a large language model.
type LLMClient interface {
	GenerateContent(ctx context.Context, prompt string) (string, error)
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	Close() error
}

// geminiClient is a client for the Google Gemini API.
type geminiClient struct {
	client         *genai.Client
	model          *genai.GenerativeModel
	embeddingModel *genai.EmbeddingModel
}

// NewGeminiClient creates a new Gemini API client.
func NewGeminiClient(ctx context.Context, cfg *config.Config) (LLMClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	// For text-only input, use the gemini-2.5-flash model
	model := client.GenerativeModel("gemini-2.5-flash")
	// For embeddings, use text-embedding-004
	embeddingModel := client.EmbeddingModel("text-embedding-004")
	return &geminiClient{client: client, model: model, embeddingModel: embeddingModel}, nil
}

// GenerateContent sends a prompt to the Gemini model and returns the generated text.
func (c *geminiClient) GenerateContent(ctx context.Context, prompt string) (string, error) {
	resp, err := c.model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Assuming the response is text
	text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("generated content is not text")
	}

	return string(text), nil
}

// GenerateEmbedding generates a vector embedding for the given text.
func (c *geminiClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	
	if resp.Embedding == nil {
		return nil, fmt.Errorf("no embedding generated")
	}

	return resp.Embedding.Values, nil
}

// Close closes the underlying Gemini client.
func (c *geminiClient) Close() error {
	return c.client.Close()
}
