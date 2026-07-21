package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "request timed out" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

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

func TestEmbeddingClientRetriesNetworkTimeout(t *testing.T) {
	requests := 0
	waits := 0
	client := &EmbeddingClient{
		apiKey:  "test-key",
		baseURL: "https://example.com/embeddings",
		model:   "test-model",
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			if requests == 1 {
				return nil, timeoutError{}
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"data":[{"embedding":[0.1,0.2]}]}`)),
			}, nil
		})},
		waitBeforeRetry: func(_ context.Context, delay time.Duration) error {
			waits++
			if delay != time.Second {
				t.Errorf("retry delay = %v, want %v", delay, time.Second)
			}
			return nil
		},
	}

	embedding, err := client.GenerateEmbedding(context.Background(), "test")
	if err != nil {
		t.Fatalf("GenerateEmbedding() error = %v", err)
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want 2", requests)
	}
	if waits != 1 {
		t.Fatalf("retry wait count = %d, want 1", waits)
	}
	if len(embedding) != 2 || embedding[0] != 0.1 || embedding[1] != 0.2 {
		t.Fatalf("embedding = %v, want [0.1 0.2]", embedding)
	}
}
