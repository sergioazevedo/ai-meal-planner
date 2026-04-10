package llm

import (
	"encoding/json"
	"testing"
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
