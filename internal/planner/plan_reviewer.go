package planner

import (
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

type planReviewerPromptData struct {
	OriginalRequest   string
	CurrentPlan       []DayPlan
	Adults            int
	Children          int
	ChildrenAges      []int
	AdjustmentFeedback string
	AvailableRecipes  []recipe.Recipe
}

type PlanReviewerResult struct {
	RevisedPlan *MealPlan
	Meta        shared.AgentMeta
}

// RunPlanReviewer revises a meal plan based on user feedback
func (p *Planner) RunPlanReviewer(
	ctx context.Context,
	currentPlan *MealPlan,
	userRequest string,
	adjustmentFeedback string,
	planningCtx PlanningContext,
	recipes []recipe.Recipe,
) (PlanReviewerResult, error) {
	start := time.Now()

	prompt, err := buildPlanReviewerPrompt(planReviewerPromptData{
		OriginalRequest:    userRequest,
		CurrentPlan:        currentPlan.Plan,
		Adults:             planningCtx.Adults,
		Children:           planningCtx.Children,
		ChildrenAges:       planningCtx.ChildrenAges,
		AdjustmentFeedback: adjustmentFeedback,
		AvailableRecipes:   recipes,
	})
	if err != nil {
		return PlanReviewerResult{}, err
	}

	resp, err := p.reviewerGenerator.GenerateContent(ctx, prompt)
	if err != nil {
		return PlanReviewerResult{}, err
	}

	result := &MealPlan{}
	rawResponse := struct {
		Plan []DayPlan `json:"plan"`
	}{}

	if err = json.Unmarshal([]byte(resp.Content), &rawResponse); err != nil {
		return PlanReviewerResult{
			Meta: shared.AgentMeta{
				AgentName: "PlanReviewer",
				Usage:     resp.Usage,
			},
		}, fmt.Errorf(
			"failed to parse plan reviewer response %w. Response: %s",
			err,
			resp.Content,
		)
	}

	// Copy over the revised plan
	result.Plan = rawResponse.Plan
	result.WeekStart = currentPlan.WeekStart
	result.Status = currentPlan.Status

	return PlanReviewerResult{
		RevisedPlan: result,
		Meta: shared.AgentMeta{
			AgentName: "PlanReviewer",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func buildPlanReviewerPrompt(data planReviewerPromptData) (string, error) {
	tmpl, err := template.New("planreviewer").Parse(planReviewerPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
