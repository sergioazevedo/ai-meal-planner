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

//go:embed analyst_prompt.md
var analystPrompt string

//go:embed user_context_prompt.md
var userContextPrompt string

type userContextData struct {
	UserRequest  string
	Recipes      []value.Recipe
	Adults       int
	Children     int
	ChildrenAges []int
}

type MealAction string

const (
	MealActionCook      MealAction = "Cook"
	MealActionLeftOvers MealAction = "Reuse"
)

type PlannedMeal struct {
	Day         string     `json:"day"`
	RecipeID    string     `json:"-"`
	Action      MealAction `json:"action"`
	RecipeTitle string     `json:"recipe_title"`
	Note        string     `json:"note"`
}

type MealProposal struct {
	PlannedMeals []PlannedMeal
	Recipes      []value.Recipe
	Adults       int
	Children     int
	ChildrenAges []int
}

type AnalystResult struct {
	Proposal *MealProposal
	Meta     shared.AgentMeta
}

type rawLlmResult struct {
	SelectedRecipesAudit []string      `json:"selected_recipes_audit"`
	PlannedMeals         []PlannedMeal `json:"planned_meals"`
}

// Analyst handles the high-reasoning logic for creating a meal schedule.
type Analyst struct {
	llm      llm.TextGenerator
	searcher shared.RecipeSearcher
}

// NewAnalyst creates a new Analyst instance.
func NewAnalyst(llm llm.TextGenerator, searcher shared.RecipeSearcher) *Analyst {
	return &Analyst{
		llm:      llm,
		searcher: searcher,
	}
}

// Run executes the Analyst agent to create a meal schedule.
func (a *Analyst) Run(
	ctx context.Context,
	userRequest string,
	planingCtx PlanningContext,
	recipesRecentlyUsed []string,
) (AnalystResult, error) {
	start := time.Now()

	// 1. Setup Prompt & State
	userContextPromptStr, err := buildUserContext(userContextData{
		UserRequest:  userRequest,
		Adults:       planingCtx.Adults,
		Children:     planingCtx.Children,
		ChildrenAges: planingCtx.ChildrenAges,
	})
	if err != nil {
		return AnalystResult{}, err
	}

	chat := llm.Conversation{{
		Role:    "system",
		Content: analystPrompt,
	}, {
		Role:    "user",
		Content: userContextPromptStr,
	}}

	initialLookup := make(map[string]value.Recipe)

	// 2. Setup Tool Handlers
	handlers := map[string]ToolHandler[[]value.Recipe]{
		searchRecipesSemanticTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeSemanticSearch(ctx, a.searcher, toolCall, recipesRecentlyUsed)
		},
		searchRecipesRandomTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeRandomSearch(ctx, a.searcher, toolCall, recipesRecentlyUsed)
		},
	}

	// 3. Execute the autonomous loop via the Engine
	resp, recipeBatches, toolMetas, err := ExecuteAgentLoop[[]value.Recipe](
		ctx,
		a.llm,
		chat,
		[]llm.Tool{
			searchRecipesSemanticTool,
			searchRecipesRandomTool,
		},
		handlers,
	)
	if err != nil {
		return AnalystResult{}, err
	}

	// 4. Update lookup from batches
	recipeLookup := initialLookup
	for _, batch := range recipeBatches {
		for _, r := range batch {
			recipeLookup[r.Title] = r
		}
	}

	// 5. Parse JSON
	raw := &rawLlmResult{}
	if err = json.Unmarshal([]byte(resp.Message.Content), raw); err != nil {
		return AnalystResult{
				Meta: shared.AgentMeta{
					AgentName: "Analyst",
					Usage:     resp.Usage,
					ToolCalls: toolMetas,
				},
			}, fmt.Errorf(
				"failed to parse analyst prompt response %w. Response: %s",
				err,
				resp.Message.Content,
			)
	}

	// 6. Map back to Domain Models
	proposal := buildMealProposal(*raw, recipeLookup, planingCtx)

	return AnalystResult{
		Proposal: proposal,
		Meta: shared.AgentMeta{
			AgentName: "Analyst",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
			ToolCalls: toolMetas,
		},
	}, nil
}

func buildMealProposal(
	raw rawLlmResult,
	recipeLookup map[string]value.Recipe,
	pCtx PlanningContext,
) *MealProposal {
	selectedRecipes := []value.Recipe{}
	seen := make(map[string]struct{})
	finalPlannedMeals := []PlannedMeal{}

	for _, meal := range raw.PlannedMeals {
		r, ok := recipeLookup[meal.RecipeTitle]
		if ok {
			meal.RecipeID = r.ID // Inject the actual ID
		}
		finalPlannedMeals = append(finalPlannedMeals, meal)

		if meal.Action != MealActionCook || !ok {
			continue
		}

		if _, alreadySeen := seen[meal.RecipeTitle]; alreadySeen {
			continue
		}

		seen[meal.RecipeTitle] = struct{}{}
		selectedRecipes = append(selectedRecipes, r)
	}

	return &MealProposal{
		PlannedMeals: finalPlannedMeals,
		Recipes:      selectedRecipes,
		Adults:       pCtx.Adults,
		Children:     pCtx.Children,
		ChildrenAges: pCtx.ChildrenAges,
	}
}

func buildUserContext(data userContextData) (string, error) {
	tmpl, err := template.New("userContext").Parse(userContextPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
