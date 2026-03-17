package llm

import (
	"ai-meal-planner/internal/shared"
	"context"
)

type Tool struct {
	Name        string
	Description string
	Parameters  ToolParameters
}

type ToolParameters struct {
	Type       ParameterType
	Properties map[string]Property
	Required   []string
}

type Property struct {
	Type        string
	Description string
}

type ParameterType string

const (
	ParameterTypeObject = "object"
)

type PropertyType string

const (
	PropertyTypeString  = "string"
	PropertyTypeNumber  = "number"
	PropertyTypeBoolean = "boolean"
	PropertyTypeInteger = "integer"
)

// ContentResponse contains the generated text and metadata like token usage.
type ContentResponse struct {
	Content   string
	Usage     shared.TokenUsage
	ToolCalls []ToolCall
}

type ToolCall struct {
	Name string
	Args map[string]any
}

type ToolResponse struct {
	Name    string         // The name of the tool that was called
	Content map[string]any // The resulting data (e.g., {"recipes": [...]})
}

type ChatSession interface {
	// SendMessage starts the conversation or sends a follow-up text.
	SendMessage(ctx context.Context, msg string) (ContentResponse, error)
	// SendToolResponse sends the result of a tool call back to the LLM so it can continue.
	SendToolResponse(ctx context.Context, responses []ToolResponse) (ContentResponse, error)
}

// TextGenerator is an interface for generating text from a prompt.
type TextGenerator interface {
	GenerateContent(ctx context.Context, prompt string, tools []Tool) (ContentResponse, error)
	StartChat(tools []Tool) ChatSession
}

// NoTools is a helper to pass an empty slice of tools to GenerateContent.
var NoTools []Tool

// EmbeddingGenerator is an interface for generating vector embeddings from text.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}
