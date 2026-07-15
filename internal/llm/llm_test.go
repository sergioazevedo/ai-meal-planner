package llm

import (
	"testing"
)

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw json",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in markdown",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json in markdown no language",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with extra text",
			input:    "Here is the plan:\n```json\n{\"key\": \"value\"}\n```\nHope you like it!",
			expected: `{"key": "value"}`,
		},
		{
			name:     "json with spaces",
			input:    "  ```json\n{\"key\": \"value\"}\n```  ",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := CleanJSON(tt.input)
			if actual != tt.expected {
				t.Errorf("CleanJSON() = %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestIsAToolCall(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		expected bool
	}{
		{
			name: "tool call with empty content",
			msg: Message{
				Content: "",
				ToolCalls: []ToolCall{
					{Name: "search_recipes"},
				},
			},
			expected: true,
		},
		{
			name: "tool call with reasoning content",
			msg: Message{
				Content: "Let me search for some pasta recipes.",
				ToolCalls: []ToolCall{
					{Name: "search_recipes"},
				},
			},
			expected: true,
		},
		{
			name: "no tool call with content",
			msg: Message{
				Content:   "I found some recipes.",
				ToolCalls: nil,
			},
			expected: false,
		},
		{
			name: "empty message",
			msg: Message{
				Content:   "",
				ToolCalls: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.msg.IsAToolCall()
			if actual != tt.expected {
				t.Errorf("%s: IsAToolCall() = %v, want %v", tt.name, actual, tt.expected)
			}
		})
	}
}
