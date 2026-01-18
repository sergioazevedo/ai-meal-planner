package llm

import "context"

// TextGenerator is an interface for generating text from a prompt.
type TextGenerator interface {
	GenerateContent(ctx context.Context, prompt string) (string, error)
}

// EmbeddingGenerator is an interface for generating vector embeddings from text.
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
}

// Closer is an interface for closing resources.
type Closer interface {
	Close() error
}

// Client combines TextGenerator, EmbeddingGenerator, and Closer.
// This is useful for backward compatibility or when a single client provides all features.
type Client interface {
	TextGenerator
	EmbeddingGenerator
	Closer
}
