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

	resp, err := c.llm.GenerateContent(ctx, llm.Conversation{{Role: "user", Content: prompt}}, llm.NoTools)
	if err != nil {
		return ChefResult{}, err
	}

	result := &MealPlan{}
	if err = json.Unmarshal([]byte(resp.Message.Content), result); err != nil {
		return ChefResult{
				Meta: shared.AgentMeta{
					Usage:     resp.Usage,
					AgentName: "Chef",
				}},
			fmt.Errorf(
				"failed to parse MealPlan %w, :%s",
				err,
				resp.Message.Content,
			)
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
			Usage:     resp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func buildChefPrompt(data *MealProposal) (string, error) {
	tmpl, err := template.New("Chef").Parse(chefPrompt)
	if err != nil {
		return "", err
	}

	// Create a compact version of recipes for the Chef to save tokens
	// Chef needs ingredients for shopping list, but doesn't need instructions.
	type ChefRecipe struct {
		ID           string   `json:"id"`
		Title        string   `json:"title"`
		Ingredients  []string `json:"ingredients"`
		PrepTime     string   `json:"prep_time"`
		Servings     string   `json:"servings"`
	}

	chefRecipes := make([]ChefRecipe, len(data.Recipes))
	for i, r := range data.Recipes {
		chefRecipes[i] = ChefRecipe{
			ID:          r.ID,
			Title:       r.Title,
			Ingredients: r.Ingredients,
			PrepTime:    r.PrepTime,
			Servings:    r.Servings,
		}
	}

	// We use an anonymous struct to pass the modified recipes list to the template
	chefData := struct {
		PlannedMeals []PlannedMeal
		Recipes      []ChefRecipe
		Adults       int
		Children     int
		ChildrenAges []int
	}{
		PlannedMeals: data.PlannedMeals,
		Recipes:      chefRecipes,
		Adults:       data.Adults,
		Children:     data.Children,
		ChildrenAges: data.ChildrenAges,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, chefData)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
