package llm

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/shared"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient is a client for the Google Gemini API.
type GeminiClient struct {
	client             *genai.Client
	modelName          string
	embeddingModelName string
}

// NewGeminiClient creates a new Gemini API client.
func NewGeminiClient(ctx context.Context, cfg *config.Config) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.GeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	// For text-only input, use the gemini-2.5-flash model
	model := client.GenerativeModel("gemini-2.5-flash")
	model.SetTemperature(0.1)
	// For embeddings, use gemini-embedding-001
	embeddingModel := client.EmbeddingModel("gemini-embedding-001")
	return &GeminiClient{client: client, model: model, embeddingModel: embeddingModel}, nil
}

// GenerateContent sends a prompt to the Gemini model and returns the generated text.
func (c *GeminiClient) GenerateContent(ctx context.Context, prompt string, tools []Tool) (ContentResponse, error) {
	model := c.client.GenerativeModel(c.modelName)
	genaiTools, err := mapToGenaiTools(tools)
	if err != nil {
		return ContentResponse{}, err
	}
	model.Tools = genaiTools

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return ContentResponse{}, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return ContentResponse{}, fmt.Errorf("no content generated")
	}

	contentResp := ContentResponse{}

	for _, part := range resp.Candidates[0].Content.Parts {
		switch p := part.(type) {
		case genai.Text:
			contentResp.Content += string(p)
		case genai.FunctionCall:
			contentResp.ToolCalls = append(contentResp.ToolCalls, ToolCall{
				Name: p.Name,
				Args: p.Args,
			})
		}
	}

	usage := shared.TokenUsage{
		Model: "gemini-2.5-flash",
	}
	if resp.UsageMetadata != nil {
		usage.PromptTokens = int(resp.UsageMetadata.PromptTokenCount)
		usage.CompletionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		usage.TotalTokens = int(resp.UsageMetadata.TotalTokenCount)
	}

	contentResp.Usage = usage

	return contentResp, nil
}

// GenerateEmbedding generates a vector embedding for the given text.
func (c *GeminiClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embeddingModel := c.client.EmbeddingModel(c.embeddingModelName)
	resp, err := embeddingModel.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if resp.Embedding == nil {
		return nil, fmt.Errorf("no embedding generated")
	}

	return resp.Embedding.Values, nil
}

// Close closes the underlying Gemini client.
func (c *GeminiClient) Close() error {
	return c.client.Close()
}
