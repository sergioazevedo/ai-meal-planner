package shared

import (
	"time"
)

// TokenUsage tracks the tokens consumed by a request.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Model            string
}

type ToolCallMeta struct {
	ToolName      string        `json:"tool_name"`
	Input         any           `json:"input"`
	OutputSummary string        `json:"output_summary"`
	Latency       time.Duration `json:"latency"`
}

// AgentMeta holds operational metadata for an agent execution.
type AgentMeta struct {
	AgentName string
	Usage     TokenUsage
	Latency   time.Duration
	ToolCalls []ToolCallMeta
}
