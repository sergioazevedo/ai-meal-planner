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

// AgentMeta holds operational metadata for an agent execution.
type AgentMeta struct {
	AgentName string
	Usage     TokenUsage
	Latency   time.Duration
}
