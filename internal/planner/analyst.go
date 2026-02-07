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

//go:embed analyst_prompt.md
var analystPrompt string

type analystPromptData struct {
	UserRequest  string
	Adults       int
	Children     int
	ChildrenAges []int
	Recipes      []recipe.Recipe
}

type MealAction string

const (
	MealActionCook      MealAction = "Cook"
	MealActionLeftOvers MealAction = "Reuse"
)

type PlannedMeal struct {
	Day         string     `json:"day"`
	RecipeID    string     `json:"recipe_id"`
	Action      MealAction `json:"action"`
	RecipeTitle string     `json:"recipe_title"`
	Note        string     `json:"note"`
}

type MealProposal struct {
	PlannedMeals []PlannedMeal
	Recipes      []recipe.Recipe
	Adults       int
	Children     int
	ChildrenAges []int
}

type AnalystResult struct {
	Proposal *MealProposal
	Meta     shared.AgentMeta
}

type rawLlmResult struct {
	PlannedMeals []PlannedMeal `json:"planned_meals"`
}

func (p *Planner) runAnalyst(
	ctx context.Context,
	userRequest string,
	planingCtx PlanningContext,
	recipes []recipe.Recipe,
) (AnalystResult, error) {
	start := time.Now()
	prompt, err := buildAnalystPrompt(analystPromptData{
		UserRequest:  userRequest,
		Adults:       planingCtx.Adults,
		Children:     planingCtx.Children,
		ChildrenAges: planingCtx.ChildrenAges,
		Recipes:      recipes,
	})
	if err != nil {
		return AnalystResult{}, err
	}

	resp, err := p.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return AnalystResult{}, err
	}

	raw := &rawLlmResult{}
	if err = json.Unmarshal([]byte(resp.Content), raw); err != nil {
		return AnalystResult{
				Meta: shared.AgentMeta{
					AgentName: "Analyst",
					Usage:     resp.Usage,
				},
			}, fmt.Errorf(
				"failed to parse analyst prompt response %w. Response: %s",
				err,
				resp.Content,
			)
	}

	recipeLookup := make(map[string]recipe.Recipe)
	for _, r := range recipes {
		recipeLookup[r.Title] = r
	}

	selectedRecipes := []recipe.Recipe{}
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

	return AnalystResult{
		Proposal: &MealProposal{
			PlannedMeals: raw.PlannedMeals,
			Recipes:      selectedRecipes,
			Adults:       planingCtx.Adults,
			Children:     planingCtx.Children,
			ChildrenAges: planingCtx.ChildrenAges,
		},
		Meta: shared.AgentMeta{
			AgentName: "Analyst",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
		},
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
