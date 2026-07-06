package llm

import (
	"ai-meal-planner/internal/shared"
	"context"
	"regexp"
	"strings"
)

var jsonRegex = regexp.MustCompile("(?s)```(?:json)?\n?(.*?)\n?```")

// CleanJSON removes markdown code blocks from a string if present.
func CleanJSON(input string) string {
	input = strings.TrimSpace(input)
	if matches := jsonRegex.FindStringSubmatch(input); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return input
}

type Tool struct {
	Name        string
	Description string
	Parameters  ToolParameters
}

type ToolParameters struct {
	Type       ParameterType       `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Items       *Property           `json:"items,omitempty"`      // Used when Type is PropertyTypeArray
	Properties  map[string]Property `json:"properties,omitempty"` // Used when Type is object
	Required    []string            `json:"required,omitempty"`   // Used when Type is object
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
	PropertyTypeArray   = "array"
	PropertyTypeObject  = "object"
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

func (m *Message) IsAToolResponse() bool {
	return m.Role == "tool"
}

// Compactor is a function that takes the raw Content of a tool response
// and returns a distilled version of it.
type Compactor func(content string) (string, error)

// Conversation is a sequence of messages representing a chat history.
type Conversation []Message

func (c Conversation) Add(msg Message) Conversation {
	return append(c, msg)
}

func (c Conversation) Compact(fn Compactor) (Conversation, error) {
	lastToolIdx := -1
	for i := len(c) - 1; i >= 0; i-- {
		if c[i].IsAToolResponse() {
			lastToolIdx = i
			break
		}
	}

	var result Conversation
	for msgIdx, msg := range c {

		// if its a regular message we keep as is
		if !msg.IsAToolResponse() {
			result = result.Add(msg)

			continue
		}

		// we don't compact the last tool Response
		if msgIdx == lastToolIdx {
			result = result.Add(msg)

			continue
		}

		// //all other tool responses will be compacted
		newContent, err := fn(msg.Content)
		if err != nil {
			return nil, err
		}

		result = append(
			result,
			Message{
				Role:       msg.Role,
				Content:    newContent,
				ToolCalls:  msg.ToolCalls,
				ToolCallID: msg.ToolCallID,
			},
		)

	}

	return result, nil
}

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
