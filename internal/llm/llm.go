package llm

import (
	"context"
	"time"
)

// TokenUsage tracks the tokens consumed by a request.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Model            string
}

// llm.AgentMeta holds operational metadata for an agent execution.
type AgentMeta struct {
	AgentName string
	Usage     TokenUsage
	Latency   time.Duration
}

// ContentResponse contains the generated text and metadata like token usage.
type ContentResponse struct {
	Content string
	Usage   TokenUsage
}

// TextGenerator is an interface for generating text from a prompt.
type TextGenerator interface {
	GenerateContent(ctx context.Context, prompt string) (ContentResponse, error)
}

// EmbeddingGenerator is an interface for generating vector embeddings from text.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Closer is an interface for closing resources.
type Closer interface {
	Close() error
}
