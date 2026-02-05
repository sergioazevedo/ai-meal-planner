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
	return &GeminiClient{
		client:             client,
		modelName:          "gemini-2.5-flash",
		embeddingModelName: "gemini-embedding-001",
	}, nil
}

// GenerateContent sends a prompt to the Gemini model and returns the generated text.
func (c *GeminiClient) GenerateContent(ctx context.Context, prompt string, tools []Tool) (ContentResponse, error) {
	model := c.client.GenerativeModel(c.modelName)
	model.SetTemperature(0.1)
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

func mapToGenaiTools(tools []Tool) ([]*genai.Tool, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	genaiTools := make([]*genai.Tool, len(tools))
	for i, t := range tools {
		properties := make(map[string]*genai.Schema)
		for key, prop := range t.Parameters.Properties {
			schemaType, err := mapToGenaiType(prop.Type)
			if err != nil {
				return nil, err
			}
			properties[key] = &genai.Schema{
				Type:        schemaType,
				Description: prop.Description,
			}
		}

		paramType, err := mapToGenaiType(string(t.Parameters.Type))
		if err != nil {
			return nil, err
		}

		genaiTools[i] = &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        t.Name,
					Description: t.Description,
					Parameters: &genai.Schema{
						Type:       paramType,
						Properties: properties,
						Required:   t.Parameters.Required,
					},
				},
			},
		}
	}
	return genaiTools, nil
}

func mapToGenaiType(t string) (genai.Type, error) {
	switch t {
	case "object":
		return genai.TypeObject, nil
	case "string":
		return genai.TypeString, nil
	case "number":
		return genai.TypeNumber, nil
	case "integer":
		return genai.TypeInteger, nil
	case "boolean":
		return genai.TypeBoolean, nil
	default:
		return -1, fmt.Errorf("unsupported schema type: %s", t)
	}
}
