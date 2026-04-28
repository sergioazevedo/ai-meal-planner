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
