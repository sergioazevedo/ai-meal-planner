package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ai-meal-planner/internal/config"
)

// EmbeddingClient is a generic HTTP client for generating vector embeddings.
// It is designed to work with APIs that follow the OpenAI-compatible /v1/embeddings format,
// such as Mixedbread AI.
type EmbeddingClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewEmbeddingClient creates a new Embedding API client.
func NewEmbeddingClient(cfg *config.Config) *EmbeddingClient {
	return &EmbeddingClient{
		apiKey:  cfg.EmbeddingAPIKey,
		baseURL: "https://api.mixedbread.com/v1/embeddings",
		// Default to Mixedbread's large model (1024 dimensions)
		model: "mixedbread-ai/mxbai-embed-large-v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type embeddingRequest struct {
	Model          string   `json:"model"`
	Input          []string `json:"input"`
	Normalized     bool     `json:"normalized"`
	EncodingFormat string   `json:"encoding_format"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// GenerateEmbedding generates a vector embedding for the given text.
func (c *EmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := embeddingRequest{
		Model:          c.model,
		Input:          []string{text},
		Normalized:     true,
		EncodingFormat: "float",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned in response")
	}

	return result.Data[0].Embedding, nil
}

func (c *EmbeddingClient) EmbeddingMetadata() EmbeddingMetadata {
	return EmbeddingMetadata{
		Model:      c.model,
		Dimensions: 1024,
	}
}

// Close is a no-op for the HTTP client but satisfies the pattern used elsewhere.
func (c *EmbeddingClient) Close() error {
	return nil
}
