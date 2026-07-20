package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddingClientRetriesRateLimit(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, `{"type":"rate_limit_error"}`, http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer server.Close()

	client := &EmbeddingClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		model:      "test-model",
		httpClient: server.Client(),
	}

	embedding, err := client.GenerateEmbedding(context.Background(), "test")
	if err != nil {
		t.Fatalf("GenerateEmbedding() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
	if len(embedding) != 2 || embedding[0] != 0.1 || embedding[1] != 0.2 {
		t.Fatalf("embedding = %v, want [0.1 0.2]", embedding)
	}
}

func TestEmbeddingClientDoesNotRetryOtherErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &EmbeddingClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		model:      "test-model",
		httpClient: server.Client(),
	}

	if _, err := client.GenerateEmbedding(context.Background(), "test"); err == nil {
		t.Fatal("GenerateEmbedding() error = nil, want API error")
	}
	if requests != 1 {
		t.Fatalf("request count = %d, want 1", requests)
	}
}
