package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ai-meal-planner/internal/config"
)

const maxEmbeddingRetries = 4

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

	for attempt := 0; ; attempt++ {
		embedding, retryAfter, err := c.sendEmbeddingRequest(ctx, jsonData)
		if err == nil {
			return embedding, nil
		}
		if retryAfter == nil || attempt >= maxEmbeddingRetries {
			return nil, err
		}
		if err := waitForRetry(ctx, *retryAfter); err != nil {
			return nil, fmt.Errorf("waiting to retry embedding request: %w", err)
		}
	}
}

func (c *EmbeddingClient) sendEmbeddingRequest(ctx context.Context, jsonData []byte) ([]float32, *time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		apiErr := fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(body))
		if resp.StatusCode != http.StatusTooManyRequests {
			return nil, nil, apiErr
		}
		delay := retryDelay(resp.Header.Get("Retry-After"))
		return nil, &delay, apiErr
	}

	var result embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, nil, fmt.Errorf("no embedding returned in response")
	}

	return result.Data[0].Embedding, nil, nil
}

func retryDelay(retryAfter string) time.Duration {
	retryAfter = strings.TrimSpace(retryAfter)
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(retryAfter); err == nil {
		if delay := time.Until(retryAt); delay > 0 {
			return delay
		}
	}
	return time.Second
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
