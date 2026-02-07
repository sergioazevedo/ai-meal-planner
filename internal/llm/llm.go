package llm

import (
	"ai-meal-planner/internal/shared" // New import
	"context"
)

// ContentResponse contains the generated text and metadata like token usage.
type ContentResponse struct {
	Content string
	Usage   shared.TokenUsage
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
