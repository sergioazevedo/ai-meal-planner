package planner

import (
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/shared"
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
	searcher RecipeSearcher
}

// NewPlanReviewer creates a new PlanReviewer instance.
func NewPlanReviewer(llm llm.TextGenerator, searcher RecipeSearcher) *PlanReviewer {
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
	recentlyUsed := recipesRecentlyUsed
	initialLookup := make(map[string]recipe.Recipe) // Used for mapping titles back to IDs

	searchHandler := func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []recipe.Recipe, error) {
		recipes, msg, err := HandleRecipeSearch(ctx, r.searcher, toolCall, recentlyUsed)
		if err != nil {
			return llm.Message{}, nil, err
		}
		// Update the exclusion list for subsequent turns
		for _, recipe := range recipes {
			recentlyUsed = append(recentlyUsed, recipe.ID)
		}
		return msg, recipes, nil
	}

	handlers := map[string]ToolHandler[[]recipe.Recipe]{
		searchRecipesTool.Name: searchHandler,
	}

	// 2. Execute the autonomous loop via the Engine
	resp, recipeBatches, toolMetas, err := ExecuteAgentLoop[[]recipe.Recipe](
		ctx,
		r.llm,
		chat,
		[]llm.Tool{searchRecipesTool},
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

	// 4. Parse JSON Response
	rawResponse := struct {
		Plan []DayPlan `json:"plan"`
	}{}

	if err = json.Unmarshal([]byte(resp.Message.Content), &rawResponse); err != nil {
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

	// 5. Build final result and map IDs
	result := &MealPlan{}
	result.Plan = []DayPlan{}
	for _, day := range rawResponse.Plan {
		// Try to find the recipe ID from our lookup
		if recipe, ok := recipeLookup[day.RecipeTitle]; ok {
			day.RecipeID = recipe.ID
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
