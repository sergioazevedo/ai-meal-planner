package llm

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/config"

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
	modelName := "gemini-2.5-flash"
	// For embeddings, use gemini-embedding-001
	embeddingModelName := "gemini-embedding-001"
	return &GeminiClient{
		client:             client,
		modelName:          modelName,
		embeddingModelName: embeddingModelName,
	}, nil
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
