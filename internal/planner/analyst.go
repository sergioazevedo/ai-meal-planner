package planner

import (
	"ai-meal-planner/internal/recipe"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed analyst_prompt.md
var analystPrompt string

type analystPromptData struct {
	UserRequest  string
	Adults       int
	Children     int
	ChildrenAges []int
	Recipes      []recipe.NormalizedRecipe
}

type MealAction string

const (
	MealActionCook      MealAction = "Cook"
	MealActionLeftOvers MealAction = "Reuse"
)

type PlannedMeal struct {
	Day         string     `json:"day"`
	Action      MealAction `json:"action"`
	RecipeTitle string     `json:"recipe_title"`
	Note        string     `json:"note"`
}

type MealProposal struct {
	PlannedMeals []PlannedMeal
	Recipes      []recipe.NormalizedRecipe
}

type rawLlmResult struct {
	PlannedMeals []PlannedMeal `json:"planned_meals"`
}

func (p *Planner) runAnalyst(
	ctx context.Context,
	userRequest string,
	planingCtx PlanningContext,
	recipes []recipe.NormalizedRecipe,
) (*MealProposal, error) {
	prompt, err := buildAnalystPrompt(analystPromptData{
		UserRequest:  userRequest,
		Adults:       planingCtx.Adults,
		Children:     planingCtx.Children,
		ChildrenAges: planingCtx.ChildrenAges,
		Recipes:      recipes,
	})
	if err != nil {
		return nil, err
	}

	llmResponse, err := p.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, err
	}

	raw := &rawLlmResult{}
	if err = json.Unmarshal([]byte(llmResponse), raw); err != nil {
		return nil, fmt.Errorf("failed to parse analyst prompt response %w. Response: %s", err, llmResponse)
	}

	recipeLookup := make(map[string]recipe.NormalizedRecipe)
	for _, r := range recipes {
		recipeLookup[r.Title] = r
	}

	selectedRecipes := []recipe.NormalizedRecipe{}
	seen := make(map[string]struct{})
	for _, meal := range raw.PlannedMeals {
		if meal.Action != MealActionCook {
			continue
		}

		r, ok := recipeLookup[meal.RecipeTitle]
		if !ok {
			continue
		}

		if _, alreadySeen := seen[meal.RecipeTitle]; alreadySeen {
			continue
		}

		seen[meal.RecipeTitle] = struct{}{}
		selectedRecipes = append(selectedRecipes, r)
	}

	return &MealProposal{
		PlannedMeals: raw.PlannedMeals,
		Recipes:      selectedRecipes,
	}, nil
}

func buildAnalystPrompt(data analystPromptData) (string, error) {
	tmpl, err := template.New("analyst").Parse(analystPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
