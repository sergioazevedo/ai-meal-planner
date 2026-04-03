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
	Message Message
	Usage   shared.TokenUsage
}

type ToolCall struct {
	ID   string
	Name string
	Args map[string]any
}

type ToolResponse struct {
	Name    string         // The name of the tool that was called
	Content map[string]any // The resulting data (e.g., {"recipes": [...]})
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

func (m *Message) IsAToolCall() bool {
	return m.Content == "" && len(m.ToolCalls) > 0
}

// Conversation is a sequence of messages representing a chat history.
type Conversation []Message

// TextGenerator is an interface for generating text from a prompt.
type TextGenerator interface {
	GenerateContent(
		ctx context.Context,
		conversation Conversation,
		tools []Tool,
	) (ContentResponse, error)
}

// NoTools is a helper to pass an empty slice of tools to GenerateContent.
var NoTools []Tool

// EmbeddingGenerator is an interface for generating vector embeddings from text.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}
