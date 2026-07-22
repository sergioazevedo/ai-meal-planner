package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMapToGroqMessages(t *testing.T) {
	conversation := []Message{
		{
			Role:    "user",
			Content: "Hello",
		},
		{
			Role: "assistant",
			ToolCalls: []ToolCall{
				{
					ID:   "call_123",
					Name: "search_recipes",
					Args: map[string]any{"query": "chicken"},
				},
			},
		},
		{
			Role:       "tool",
			Content:    `{"recipes": []}`,
			ToolCallID: "call_123",
		},
	}

	groqMsgs, err := mapToGroqMessages(conversation)
	if err != nil {
		t.Fatalf("mapToGroqMessages failed: %v", err)
	}

	if len(groqMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(groqMsgs))
	}

	// Check assistant message
	if groqMsgs[1].Role != "assistant" {
		t.Errorf("expected role assistant, got %s", groqMsgs[1].Role)
	}
	if len(groqMsgs[1].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(groqMsgs[1].ToolCalls))
	}
	if groqMsgs[1].ToolCalls[0].ID != "call_123" {
		t.Errorf("expected tool call ID call_123, got %s", groqMsgs[1].ToolCalls[0].ID)
	}

	// Check tool message
	if groqMsgs[2].Role != "tool" {
		t.Errorf("expected role tool, got %s", groqMsgs[2].Role)
	}
	if groqMsgs[2].ToolCallID != "call_123" {
		t.Errorf("expected tool_call_id call_123, got %s", groqMsgs[2].ToolCallID)
	}

	// Verify JSON marshaling
	bytes, err := json.Marshal(groqMsgs)
	if err != nil {
		t.Fatalf("failed to marshal groq messages: %v", err)
	}

	var raw []map[string]any
	if err := json.Unmarshal(bytes, &raw); err != nil {
		t.Fatalf("failed to unmarshal groq messages: %v", err)
	}

	toolMsg := raw[2]
	if toolMsg["tool_call_id"] != "call_123" {
		t.Errorf("expected tool_call_id in JSON to be call_123, got %v", toolMsg["tool_call_id"])
	}
}

func TestMapToTToolCall(t *testing.T) {
	client := &GroqClient{}
	rawCall := groqToolCall{
		ID:   "call_456",
		Type: "function",
	}
	rawCall.Function.Name = "test_tool"
	rawCall.Function.Arguments = `{"arg1": "val1"}`

	mapped, err := client.mapToTToolCall(rawCall)
	if err != nil {
		t.Fatalf("mapToTToolCall failed: %v", err)
	}

	if mapped.ID != "call_456" {
		t.Errorf("expected ID call_456, got %s", mapped.ID)
	}
	if mapped.Name != "test_tool" {
		t.Errorf("expected name test_tool, got %s", mapped.Name)
	}
}

func TestGroqRetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		body    string
		want    time.Duration
	}{
		{
			name:    "retry-after header takes priority",
			headers: http.Header{"Retry-After": []string{"2"}, "X-Ratelimit-Reset-Tokens": []string{"500ms"}},
			want:    2100 * time.Millisecond,
		},
		{
			name:    "token reset header",
			headers: http.Header{"X-Ratelimit-Reset-Tokens": []string{"750ms"}},
			want:    850 * time.Millisecond,
		},
		{
			name: "millisecond response hint",
			body: `{"error":{"message":"Please try again in 600ms."}}`,
			want: 700 * time.Millisecond,
		},
		{name: "fallback", want: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := groqRetryDelay(tt.headers, tt.body); got != tt.want {
				t.Fatalf("groqRetryDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReasoningEffortForModel(t *testing.T) {
	tests := map[string]string{
		"openai/gpt-oss-120b":  "low",
		"openai/gpt-oss-20b":   "low",
		"qwen/qwen3.6-27b":     "none",
		"llama-3.1-8b-instant": "",
	}
	for model, want := range tests {
		if got := reasoningEffortForModel(model); got != want {
			t.Errorf("reasoningEffortForModel(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestGroqClientRetriesUsingRateLimitHeaders(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"{}"}}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`))
	}))
	defer server.Close()

	client := &GroqClient{
		apiKey:     "test",
		modelID:    ModelNormalizer,
		apiURL:     server.URL,
		httpClient: server.Client(),
	}
	response, err := client.GenerateContent(
		context.Background(),
		Conversation{{Role: "user", Content: "test"}},
		NoTools,
	)
	if err != nil {
		t.Fatalf("GenerateContent() error = %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
	if response.Usage.TotalTokens != 2 {
		t.Fatalf("total tokens = %d, want 2", response.Usage.TotalTokens)
	}
}
