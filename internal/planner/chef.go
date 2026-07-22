package planner

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

//go:embed chef_prompt.md
var chefPrompt string

const maxChefAttempts = 2

type ChefResult struct {
	Plan *MealPlan
	Meta shared.AgentMeta
}

// Chef handles the generation of the final MealPlan and shopping list.
type Chef struct {
	llm llm.TextGenerator
}

// NewChef creates a new Chef instance.
func NewChef(llm llm.TextGenerator) *Chef {
	return &Chef{
		llm: llm,
	}
}

// Run executes the Chef agent to generate a meal plan.
func (c *Chef) Run(
	ctx context.Context,
	mealSchedule *MealProposal,
	weekStart time.Time,
) (ChefResult, error) {
	start := time.Now()
	prompt, err := buildChefPrompt(mealSchedule)
	if err != nil {
		return ChefResult{}, err
	}

	conversation := llm.Conversation{{Role: "user", Content: prompt}}
	var usage shared.TokenUsage
	var result *MealPlan
	var lastErr error
	var lastContent string
	for attempt := 0; attempt < maxChefAttempts; attempt++ {
		resp, err := c.llm.GenerateContent(ctx, conversation, llm.NoTools)
		if err != nil {
			return ChefResult{}, err
		}
		usage.PromptTokens += resp.Usage.PromptTokens
		usage.CompletionTokens += resp.Usage.CompletionTokens
		usage.TotalTokens += resp.Usage.TotalTokens
		usage.Model = resp.Usage.Model

		result = &MealPlan{}
		lastContent = resp.Message.Content
		lastErr = json.Unmarshal([]byte(llm.CleanJSON(lastContent)), result)
		if lastErr == nil {
			break
		}
		conversation = conversation.Add(resp.Message).Add(llm.Message{
			Role:    "user",
			Content: "The response was invalid JSON. Return the corrected raw JSON object only.",
		})
	}
	if lastErr != nil {
		return ChefResult{Meta: shared.AgentMeta{Usage: usage, AgentName: "Chef"}},
			fmt.Errorf("failed to parse MealPlan %w, :%s", lastErr, lastContent)
	}

	result.WeekStart = weekStart

	// Post-processing: Map IDs back from the Analyst's proposal
	// The Chef might have modified the titles (e.g., adding "Cook:" prefix),
	// so we use the original proposal's order which is preserved (9 meals).
	if len(result.Plan) == len(mealSchedule.PlannedMeals) {
		for i := range result.Plan {
			result.Plan[i].RecipeID = mealSchedule.PlannedMeals[i].RecipeID
		}
	}

	return ChefResult{
		Plan: result,
		Meta: shared.AgentMeta{
			AgentName: "Chef",
			Usage:     usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func buildChefPrompt(data *MealProposal) (string, error) {
	tmpl, err := template.New("Chef").Parse(chefPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
