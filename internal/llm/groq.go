package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	apiKey      string
	modelID     string
	temperature float64
	httpClient  *http.Client
}

type groqTool struct {
	Type     string       `json:"type"`
	Function groqFunction `json:"function"`
}

type groqFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type groqMessage struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []groqToolCall `json:"tool_calls,omitempty"`
}

type groqResponseFormat struct {
	Type string `json:"type,omitempty"`
}

type groqRequest struct {
	Model          string              `json:"model,omitempty"`
	Messages       []groqMessage       `json:"messages,omitempty"`
	Tools          []groqTool          `json:"tools,omitempty"`
	ToolChoice     string              `json:"tool_choice,omitempty"`
	Temperature    float64             `json:"temperature,omitempty"`
	ResponseFormat *groqResponseFormat `json:"response_format,omitempty"`
}

type groqToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type groqTokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type groqChoices struct {
	Message groqMessage `json:"message"`
}

type groqResponse struct {
	Choices []groqChoices  `json:"choices"`
	Usage   groqTokenUsage `json:"usage"`
}

// NewGroqClient creates a new Groq API client for a specific model and temperature.
func NewGroqClient(cfg *config.Config, modelID string, temperature float64) *GroqClient {
	return &GroqClient{
		apiKey:      cfg.GroqAPIKey,
		modelID:     modelID,
		temperature: temperature,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateContent sends a prompt to the Groq model and returns the generated text.
func (c *GroqClient) GenerateContent(
	ctx context.Context,
	conversation Conversation,
	tools []Tool,
) (ContentResponse, error) {
	maxRetries := 3
	var lastErr error

	messages, err := mapToGroqMessages(conversation)
	if err != nil {
		return ContentResponse{}, err
	}

	reqBody := groqRequest{
		Model:       c.modelID,
		Messages:    messages,
		Temperature: c.temperature,
	}

	if len(tools) > 0 {
		reqBody.Tools = mapToGroqTools(tools)
		reqBody.ToolChoice = "auto"
	} else {
		reqBody.ResponseFormat = &groqResponseFormat{Type: "json_object"}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return ContentResponse{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	for i := 0; i < maxRetries; i++ {
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

		if resp.StatusCode == http.StatusTooManyRequests {
			bodyBytes, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("groq api rate limit: %s", string(bodyBytes))

			// Attempt to parse wait time from message: "Please try again in 9.24s"
			waitTime := 5 * time.Second // default
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "try again in ") {
				var seconds float64
				parts := strings.Split(bodyStr, "try again in ")
				if len(parts) > 1 {
					fmt.Sscanf(parts[1], "%fs", &seconds)
					if seconds > 0 {
						waitTime = time.Duration(seconds*1000+500) * time.Millisecond // Add 500ms buffer
					}
				}
			}

			fmt.Printf("Rate limit hit. Waiting %v before retry %d/%d...\n", waitTime, i+1, maxRetries)
			select {
			case <-ctx.Done():
				return ContentResponse{}, ctx.Err()
			case <-time.After(waitTime):
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return ContentResponse{}, fmt.Errorf("groq api error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
		}

		var groqResp groqResponse
		if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
			return ContentResponse{}, fmt.Errorf("failed to decode response: %w", err)
		}

		if len(groqResp.Choices) == 0 {
			return ContentResponse{}, fmt.Errorf("no content generated")
		}

		var toolCalls []ToolCall
		for _, call := range groqResp.Choices[0].Message.ToolCalls {
			mapped, err := c.mapToTToolCall(call)
			if err != nil {
				return ContentResponse{}, err
			}

			toolCalls = append(toolCalls, mapped)
		}

		return ContentResponse{
			Message: Message{
				Role:      groqResp.Choices[0].Message.Role,
				Content:   groqResp.Choices[0].Message.Content,
				ToolCalls: toolCalls,
			},
			Usage: shared.TokenUsage{
				PromptTokens:     groqResp.Usage.PromptTokens,
				CompletionTokens: groqResp.Usage.CompletionTokens,
				TotalTokens:      groqResp.Usage.TotalTokens,
				Model:            c.modelID,
			},
		}, nil
	}

	return ContentResponse{}, fmt.Errorf("exceeded max retries after rate limit: %w", lastErr)
}

func (c *GroqClient) mapToTToolCall(call groqToolCall) (ToolCall, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return ToolCall{}, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	return ToolCall{
		Name: call.Function.Name,
		Args: args,
	}, nil
}

func mapToGroqTools(tools []Tool) []groqTool {
	if len(tools) == 0 {
		return nil
	}

	var gropTools []groqTool
	for _, t := range tools {
		gropTools = append(gropTools, groqTool{
			Type: "function",
			Function: groqFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return gropTools
}

func mapToGroqMessages(conversation []Message) ([]groqMessage, error) {
	var result []groqMessage
	for _, m := range conversation {
		calls, err := mapToGroqToolCalls(m.ToolCalls)
		if err != nil {
			return nil, err
		}

		result = append(result, groqMessage{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: calls,
		})
	}
	return result, nil
}

func mapToGroqToolCalls(calls []ToolCall) ([]groqToolCall, error) {
	var result []groqToolCall
	for _, c := range calls {
		var argBytes, err = json.Marshal(c.Args)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize tool args: %w", err)
		}

		result = append(result, groqToolCall{
			ID:   c.ID,
			Type: "function",
			Function: struct {
				Name      string "json:\"name\""
				Arguments string "json:\"arguments\""
			}{
				Name:      c.Name,
				Arguments: string(argBytes),
			},
		})
	}
	return result, nil
}
