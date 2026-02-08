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
	"ai-meal-planner/internal/shared"
)

const (
	groqAPIURL = "https://api.groq.com/openai/v1/chat/completions"

	// Model identifiers
	ModelAnalyst    = "llama-3.3-70b-versatile"
	ModelNormalizer = "llama-3.1-8b-instant"
)

// GroqClient is a client for the Groq API.
type GroqClient struct {
	apiKey     string
	modelID    string
	httpClient *http.Client
}

// NewGroqClient creates a new Groq API client for a specific model.
func NewGroqClient(cfg *config.Config, modelID string) *GroqClient {
	return &GroqClient{
		apiKey:  cfg.GroqAPIKey,
		modelID: modelID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateContent sends a prompt to the Groq model and returns the generated text.
func (c *GroqClient) GenerateContent(ctx context.Context, prompt string) (ContentResponse, error) {
	reqBody := map[string]interface{}{
		"model": c.modelID,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature":     0.1,
		"response_format": map[string]string{"type": "json_object"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", groqAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return ContentResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return ContentResponse{}, fmt.Errorf("groq api error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return ContentResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return ContentResponse{}, fmt.Errorf("no content generated")
	}

	return ContentResponse{
		Content: groqResp.Choices[0].Message.Content,
		Usage: shared.TokenUsage{
			PromptTokens:     groqResp.Usage.PromptTokens,
			CompletionTokens: groqResp.Usage.CompletionTokens,
			TotalTokens:      groqResp.Usage.TotalTokens,
			Model:            c.modelID,
		},
	}, nil
}
