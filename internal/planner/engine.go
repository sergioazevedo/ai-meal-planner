package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	const maxTurns = 15
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

		chat = chat.Add(resp.Message)
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

			chat = chat.Add(msg)
			if msg.IsAToolResponse() {
				chat, err = chat.Compact(recipeCompactor)
				if err != nil {
					return llm.ContentResponse{}, nil, nil, err
				}
			}
			sideEffects = append(sideEffects, effect)
		}
	}

	return resp, sideEffects, metas, nil
}

var recipeCompactor = func(content string) (string, error) {
	var recipes []value.Recipe
	if err := json.Unmarshal([]byte(content), &recipes); err != nil {
		// If the tool response doesn't have Recipes
		// don't fail! Just return the original content unchanged.
		if content == "" {
			return "No results", nil
		}
		return content, nil
	}

	if len(recipes) == 0 {
		return "No recipes found matching the criteria", nil
	}

	var data []string
	for _, r := range recipes {
		data = append(
			data,
			fmt.Sprintf("%s (%s)", r.Title, r.PrepTime),
		)
	}

	return strings.Join(data, ","), nil
}

var searchRecipesSemanticTool = llm.Tool{
	Name:        "search_recipes_semantic",
	Description: "Search for specific recipes based on a query. Use this tool when the user has specific requests, dietary needs, cuisines, or ingredients they want to include or avoid.",
	Parameters: llm.ToolParameters{
		Type: llm.ParameterTypeObject,
		Properties: map[string]llm.Property{
			"query": {
				Type:        llm.PropertyTypeString,
				Description: "The search query (e.g., 'spicy chicken', 'low carb', 'Italian').",
			},
			"exclude_tags": {
				Type:        llm.PropertyTypeArray,
				Description: "A list of tags (in English) to completely exclude from the search (e.g., ['chicken', 'beef', 'dairy']). Use this to enforce negative constraints.",
				Items: &llm.Property{
					Type: llm.PropertyTypeString,
				},
			},
			"reasoning": {
				Type:        llm.PropertyTypeString,
				Description: "A brief explanation of why you are running this search and what you hope to find based on previous results.",
			},
		},
		Required: []string{"query", "reasoning"},
	},
}

// simplifyForTool reduces the payload of a recipe array to minimize token usage in the LLM context window.
func simplifyForTool(recipes []value.Recipe) []value.Recipe {
	content := []value.Recipe{} // Initialize to avoid nil marshaling to "null" if preferred, though "[]" is better
	for _, r := range recipes {
		content = append(content, value.Recipe{
			ID:       r.ID,
			Title:    r.Title,
			PrepTime: r.PrepTime,
			Tags:     r.Tags,
			Servings: r.Servings,
		})
	}
	return content
}

// HandleRecipeSemanticSearch executes the search_recipes tool and formats the result as an LLM message.
func HandleRecipeSemanticSearch(
	ctx context.Context,
	searcher shared.RecipeSearcher,
	toolCall llm.ToolCall,
	recipesRecentlyUsed []string,
) (llm.Message, []value.Recipe, error) {
	var excludeTags []string
	if tags, ok := toolCall.Args["exclude_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if strTag, ok := tag.(string); ok {
				excludeTags = append(excludeTags, strTag)
			}
		}
	}

	recipes, err := searcher.RecipeSemanticSearch(
		ctx,
		toolCall.Args["query"].(string),
		recipesRecentlyUsed,
		excludeTags, // Passed down!
	)
	if err != nil {
		return llm.Message{}, nil, err
	}

	recipesJson, err := json.Marshal(simplifyForTool(recipes))
	if err != nil {
		return llm.Message{}, nil, err
	}

	msg := llm.Message{
		Role:       "tool",
		Content:    string(recipesJson),
		ToolCallID: toolCall.ID,
	}

	return msg, recipes, nil
}

var searchRecipesRandomTool = llm.Tool{
	Name:        "search_recipes_random",
	Description: "Discover random recipes. Use this tool when the request is generic (e.g., 'plan for the week') or when you need to introduce variety and unexpected options into the meal plan.",
	Parameters: llm.ToolParameters{
		Type: llm.ParameterTypeObject,
		Properties: map[string]llm.Property{
			"limit": {
				Type:        llm.PropertyTypeNumber,
				Description: "The number of random recipes to retrieve. Default is 10.",
			},
			"exclude_tags": {
				Type:        llm.PropertyTypeArray,
				Description: "A list of tags (in English) to completely exclude from the search (e.g., ['chicken', 'beef', 'dairy']). Use this to enforce negative constraints.",
				Items: &llm.Property{
					Type: llm.PropertyTypeString,
				},
			},
			"reasoning": {
				Type:        llm.PropertyTypeString,
				Description: "A brief explanation of why you are running this search and what you hope to find based on previous results.",
			},
		},
		Required: []string{"limit", "reasoning"},
	},
}

func HandleRecipeRandomSearch(
	ctx context.Context,
	searcher shared.RecipeSearcher,
	toolCall llm.ToolCall,
	recipesRecentlyUsed []string,
) (llm.Message, []value.Recipe, error) {
	limit := int64(10)
	if val, ok := toolCall.Args["limit"].(float64); ok {
		limit = int64(val)
	}

	var excludeTags []string
	if tags, ok := toolCall.Args["exclude_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if strTag, ok := tag.(string); ok {
				excludeTags = append(excludeTags, strTag)
			}
		}
	}

	recipes, err := searcher.RandomRecipes(
		ctx,
		limit,
		recipesRecentlyUsed,
		excludeTags, // Passed down!
	)
	if err != nil {
		return llm.Message{}, nil, err
	}

	recipesJson, err := json.Marshal(simplifyForTool(recipes))
	if err != nil {
		return llm.Message{}, nil, err
	}

	msg := llm.Message{
		Role:       "tool",
		Content:    string(recipesJson),
		ToolCallID: toolCall.ID,
	}

	return msg, recipes, nil
}
