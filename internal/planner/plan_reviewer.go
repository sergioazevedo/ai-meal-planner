package planner

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"ai-meal-planner/internal/value"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

//go:embed plan_reviewer_prompt.md
var planReviewerPrompt string

//go:embed plan_reviewer_user_context_prompt.md
var planReviewerUserContextPrompt string

var submitRevisedPlanTool = llm.Tool{
	Name:        "submit_revised_plan",
	Description: "Submit the revised meal plan. Use this tool ONLY when you have successfully adjusted the meal plan according to the user feedback. This is your final action.",
	Parameters: llm.ToolParameters{
		Type: llm.ParameterTypeObject,
		Properties: map[string]llm.Property{
			"plan": {
				Type:        llm.PropertyTypeArray,
				Description: "The revised meal schedule.",
				Items: &llm.Property{
					Type: llm.PropertyTypeObject,
					Properties: map[string]llm.Property{
						"day": {
							Type: llm.PropertyTypeString,
						},
						"recipe_title": {
							Type: llm.PropertyTypeString,
						},
						"note": {
							Type: llm.PropertyTypeString,
						},
					},
					Required: []string{"day", "recipe_title", "note"},
				},
			},
		},
		Required: []string{"plan"},
	},
}

type planReviewerUserContextData struct {
	OriginalRequest    string
	CurrentPlan        []DayPlan
	Adults             int
	Children           int
	ChildrenAges       []int
	AdjustmentFeedback string
}

type PlanReviewerResult struct {
	RevisedPlan *MealPlan
	Meta        shared.AgentMeta
}

// PlanReviewer handles the revision of an existing meal plan based on user feedback.
type PlanReviewer struct {
	llm      llm.TextGenerator
	searcher shared.RecipeSearcher
}

// NewPlanReviewer creates a new PlanReviewer instance.
func NewPlanReviewer(llm llm.TextGenerator, searcher shared.RecipeSearcher) *PlanReviewer {
	return &PlanReviewer{
		llm:      llm,
		searcher: searcher,
	}
}

// Run revises a meal plan based on user feedback.
func (r *PlanReviewer) Run(
	ctx context.Context,
	currentPlan *MealPlan,
	userRequest string,
	adjustmentFeedback string,
	planningCtx PlanningContext,
	recipesRecentlyUsed []string,
) (PlanReviewerResult, error) {
	start := time.Now()

	prompt, err := buildPlanReviewerUserContext(planReviewerUserContextData{
		OriginalRequest:    userRequest,
		CurrentPlan:        currentPlan.Plan,
		Adults:             planningCtx.Adults,
		Children:           planningCtx.Children,
		ChildrenAges:       planningCtx.ChildrenAges,
		AdjustmentFeedback: adjustmentFeedback,
	})
	if err != nil {
		return PlanReviewerResult{}, err
	}

	chat := llm.Conversation{{
		Role:    "system",
		Content: planReviewerPrompt,
	}, {
		Role:    "user",
		Content: prompt,
	}}

	// 1. Setup Tool Handlers
	initialLookup := make(map[string]value.Recipe) // Used for mapping titles back to IDs
	for _, day := range currentPlan.Plan {
		initialLookup[day.RecipeTitle] = value.Recipe{
			ID:         day.RecipeID,
			Title:      day.RecipeTitle,
			SideDishes: day.SideDishes,
			PrepTime:   day.PrepTime,
		}
	}

	rawResponse := struct {
		Plan []DayPlan `json:"plan"`
	}{}

	// 2. Setup Tool Handlers
	handlers := map[string]ToolHandler[[]value.Recipe]{
		searchRecipesSemanticTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeSemanticSearch(ctx, r.searcher, toolCall, recipesRecentlyUsed)
		},
		searchRecipesRandomTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeRandomSearch(ctx, r.searcher, toolCall, recipesRecentlyUsed)
		},
		submitRevisedPlanTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			b, err := json.Marshal(toolCall.Args)
			if err != nil {
				return llm.Message{}, nil, fmt.Errorf("failed to marshal terminal tool args: %w", err)
			}
			if err := json.Unmarshal(b, &rawResponse); err != nil {
				return llm.Message{}, nil, fmt.Errorf("failed to parse terminal tool args: %w", err)
			}
			return llm.Message{
				Role:       "tool",
				Content:    `{"status":"success"}`,
				ToolCallID: toolCall.ID,
			}, nil, ErrAgentFinished
		},
	}

	// 2. Execute the autonomous loop via the Engine
	resp, recipeBatches, toolMetas, err := ExecuteAgentLoop[[]value.Recipe](
		ctx,
		r.llm,
		chat,
		[]llm.Tool{
			searchRecipesSemanticTool,
			searchRecipesRandomTool,
			submitRevisedPlanTool,
		},
		handlers,
	)
	if err != nil {
		return PlanReviewerResult{}, err
	}

	// 3. Build lookup from retrieved recipes
	recipeLookup := initialLookup
	for _, batch := range recipeBatches {
		for _, recipe := range batch {
			recipeLookup[recipe.Title] = recipe
		}
	}

	// 4. Parse JSON Response (Fallback to message content if terminal tool wasn't called)
	if len(rawResponse.Plan) == 0 {
		cleanedJSON := llm.CleanJSON(resp.Message.Content)
		if err = json.Unmarshal([]byte(cleanedJSON), &rawResponse); err != nil {
			return PlanReviewerResult{
					Meta: shared.AgentMeta{
						AgentName: "PlanReviewer",
						Usage:     resp.Usage,
						ToolCalls: toolMetas,
					},
				}, fmt.Errorf(
					"failed to parse plan reviewer response %w. Response: %s",
					err,
					resp.Message.Content,
				)
		}
	}

	// 5. Build final result and map IDs
	result := &MealPlan{}
	revisedByDay := make(map[string]DayPlan, len(rawResponse.Plan))
	for _, day := range rawResponse.Plan {
		revisedByDay[day.Day] = day
	}
	for _, original := range currentPlan.Plan {
		day, ok := revisedByDay[original.Day]
		if !ok {
			return PlanReviewerResult{}, fmt.Errorf("revised plan is missing %s", original.Day)
		}
		if recipe, ok := recipeLookup[day.RecipeTitle]; ok {
			day.RecipeID = recipe.ID
			if day.PrepTime == "" {
				day.PrepTime = recipe.PrepTime
			}
			if len(day.SideDishes) == 0 {
				day.SideDishes = recipe.SideDishes
			}
		} else {
			return PlanReviewerResult{}, fmt.Errorf(
				"revised plan references unknown recipe %q for %s",
				day.RecipeTitle,
				original.Day,
			)
		}
		result.Plan = append(result.Plan, day)
	}

	result.WeekStart = currentPlan.WeekStart
	result.Status = currentPlan.Status
	result.OriginalRequest = userRequest

	return PlanReviewerResult{
		RevisedPlan: result,
		Meta: shared.AgentMeta{
			AgentName: "PlanReviewer",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
			ToolCalls: toolMetas,
		},
	}, nil
}

func buildPlanReviewerUserContext(
	data planReviewerUserContextData,
) (string, error) {
	tmpl, err := template.New("planreviewerusercontext").Parse(planReviewerUserContextPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
