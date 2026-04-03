package llmtest

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/llm"
)

// MockTextGenerator is a reusable mock for testing text generation.
type MockTextGenerator struct {
	Response    string
	ShouldError bool
	// GenerateFn allows tests to provide custom response logic based on the prompt.
	GenerateFn    func(conversation llm.Conversation) llm.ContentResponse
	ResponseChain []llm.ContentResponse
	callCount     int
}

func (m *MockTextGenerator) GenerateContent(
	ctx context.Context,
	conversation llm.Conversation,
	tools []llm.Tool,
) (llm.ContentResponse, error) {
	if m.ShouldError {
		return llm.ContentResponse{}, fmt.Errorf("mock ai error")
	}

	if m.GenerateFn != nil {
		return m.GenerateFn(conversation), nil
	}

	if len(m.ResponseChain) > 0 {
		if m.callCount >= len(m.ResponseChain) {
			return llm.ContentResponse{}, fmt.Errorf("too many calls to GenerateContent")
		}
		resp := m.ResponseChain[m.callCount]
		m.callCount++
		return resp, nil
	}

	return llm.ContentResponse{Message: llm.Message{
		Role:    "assistant",
		Content: m.Response,
	}}, nil
}

// MockEmbeddingGenerator is a reusable mock for testing vector embeddings.
type MockEmbeddingGenerator struct {
	ShouldError bool
	// Values allows tests to provide custom embedding results.
	Values []float32
}

func (m *MockEmbeddingGenerator) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("mock ai error")
	}
	if m.Values != nil {
		return m.Values, nil
	}
	return []float32{0.1, 0.2, 0.3}, nil
}
