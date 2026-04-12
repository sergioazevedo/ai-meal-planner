package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"ai-meal-planner/internal/value"
)

// ToolHandler is a generic function that processes a tool call and returns a side effect of type T.
type ToolHandler[T any] func(ctx context.Context, call llm.ToolCall) (llm.Message, T, error)

// ExecuteAgentLoop runs the stateless multi-turn conversation loop.
// It uses generics (T) to strongly type the side effects accumulated from tool calls.
func ExecuteAgentLoop[T any](
	ctx context.Context,
	generator llm.TextGenerator,
	chat llm.Conversation,
	tools []llm.Tool,
	handlers map[string]ToolHandler[T],
) (llm.ContentResponse, []T, []shared.ToolCallMeta, error) {

	var resp llm.ContentResponse
	var err error
	var metas []shared.ToolCallMeta
	var sideEffects []T

	const maxTurns = 5
	turnCount := 0

	for {
		if turnCount >= maxTurns {
			return llm.ContentResponse{}, nil, nil, fmt.Errorf("agent exceeded maximum tool execution turns (%d)", maxTurns)
		}
		turnCount++

		resp, err = generator.GenerateContent(ctx, chat, tools)
		if err != nil {
			return llm.ContentResponse{}, nil, nil, err
		}

		chat = append(chat, resp.Message)
		if !resp.Message.IsAToolCall() {
			break // Loop complete, we have final text/JSON
		}

		// Handle all tool calls in this turn (Note: we currently handle the first one,
		// but the loop is ready for parallel tool calls in the future)
		for _, toolCall := range resp.Message.ToolCalls {
			handler, ok := handlers[toolCall.Name]
			if !ok {
				return llm.ContentResponse{}, nil, nil, fmt.Errorf("tool not supported: %s", toolCall.Name)
			}

			start := time.Now()
			msg, effect, err := handler(ctx, toolCall)
			if err != nil {
				return llm.ContentResponse{}, nil, nil, err
			}

			metas = append(metas, shared.ToolCallMeta{
				ToolName: toolCall.Name,
				Input:    toolCall.Args,
				Latency:  time.Since(start),
			})

			chat = append(chat, msg)
			sideEffects = append(sideEffects, effect)
		}
	}

	return resp, sideEffects, metas, nil
}

var searchRecipesTool = llm.Tool{
	Name:        "search_recipes",
	Description: "Search for recipes based on a query to find meals that fit the user's requirements.",
	Parameters: llm.ToolParameters{
		Type: llm.ParameterTypeObject,
		Properties: map[string]llm.Property{
			"query": {
				Type:        llm.PropertyTypeString,
				Description: "The search query (e.g., 'chicken dinner', 'quick vegetarian').",
			},
		},
		Required: []string{"query"},
	},
}

// HandleRecipeSearch executes the search_recipes tool and formats the result as an LLM message.
func HandleRecipeSearch(
	ctx context.Context,
	searcher shared.RecipeSearcher,
	toolCall llm.ToolCall,
	recipesRecentlyUsed []string,
) ([]value.Recipe, llm.Message, error) {
	recipes, err := searcher.RecipeSemanticSearch(
		ctx,
		toolCall.Args["query"].(string),
		recipesRecentlyUsed,
	)
	if err != nil {
		return nil, llm.Message{}, err
	}

	recipesJson, err := json.Marshal(recipes)
	if err != nil {
		return nil, llm.Message{}, err
	}

	msg := llm.Message{
		Role:       "tool",
		Content:    string(recipesJson),
		ToolCallID: toolCall.ID,
	}

	return recipes, msg, nil
}
