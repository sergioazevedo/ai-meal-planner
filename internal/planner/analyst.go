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

//go:embed analyst_prompt.md
var analystPrompt string

//go:embed user_context_prompt.md
var userContextPrompt string

type userContextData struct {
	UserRequest  string
	Recipes      []recipe.Recipe
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
	SelectedRecipesAudit []string      `json:"selected_recipes_audit"`
	PlannedMeals         []PlannedMeal `json:"planned_meals"`
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

func (p *Planner) runAnalyst(
	ctx context.Context,
	userRequest string,
	planingCtx PlanningContext,
	recipePool []recipe.Recipe,
	recipesRecentlyUsed []string,
) (AnalystResult, error) {
	start := time.Now()

	// 1. Setup Prompt & State
	userContextPromptStr, err := buildUserContext(userContextData{
		UserRequest:  userRequest,
		Recipes:      recipePool,
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

	initialLookup := make(map[string]recipe.Recipe)
	for _, r := range recipePool {
		initialLookup[r.Title] = r
	}

	// 2. Execute the autonomous loop
	resp, recipeLookup, err := p.executeAnalystLoop(ctx, chat, initialLookup, recipesRecentlyUsed)
	if err != nil {
		return AnalystResult{}, err
	}

	// 3. Parse JSON
	raw := &rawLlmResult{}
	if err = json.Unmarshal([]byte(resp.Message.Content), raw); err != nil {
		return AnalystResult{
			Meta: shared.AgentMeta{
				AgentName: "Analyst",
				Usage:     resp.Usage,
			},
		}, fmt.Errorf(
			"failed to parse analyst prompt response %w. Response: %s",
			err,
			resp.Message.Content,
		)
	}

	// 4. Map back to Domain Models
	proposal := buildMealProposal(*raw, recipeLookup, planingCtx)

	return AnalystResult{
		Proposal: proposal,
		Meta: shared.AgentMeta{
			AgentName: "Analyst",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func (p *Planner) executeAnalystLoop(
	ctx context.Context,
	chat llm.Conversation,
	initialLookup map[string]recipe.Recipe,
	recipesRecentlyUsed []string,
) (llm.ContentResponse, map[string]recipe.Recipe, error) {
	tools := []llm.Tool{searchRecipesTool}
	recipeLookup := initialLookup
	var resp llm.ContentResponse
	var err error

	for {
		resp, err = p.analystGenerator.GenerateContent(
			ctx,
			chat,
			tools,
		)
		if err != nil {
			return llm.ContentResponse{}, nil, err
		}

		chat = append(chat, resp.Message)
		if !resp.Message.IsAToolCall() {
			break // Loop complete, we have final JSON
		}

		toolCall := resp.Message.ToolCalls[0]
		if toolCall.Name != "search_recipes" {
			return llm.ContentResponse{}, nil, fmt.Errorf("tool not supported %s", toolCall.Name)
		}
		
		recipes, msg, err := p.handleSearchTool(ctx, toolCall, recipesRecentlyUsed)
		if err != nil {
			return llm.ContentResponse{}, nil, err
		}

		chat = append(chat, msg)
		for _, r := range recipes {
			recipeLookup[r.Title] = r
			recipesRecentlyUsed = append(recipesRecentlyUsed, r.ID)
		}
	}

	return resp, recipeLookup, nil
}

func buildMealProposal(
	raw rawLlmResult,
	recipeLookup map[string]recipe.Recipe,
	pCtx PlanningContext,
) *MealProposal {
	selectedRecipes := []recipe.Recipe{}
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

func (p *Planner) handleSearchTool(
	ctx context.Context,
	toolCall llm.ToolCall,
	recipesRecentlyUsed []string,
) ([]recipe.Recipe, llm.Message, error) {
	recipes, err := p.getRecipeCandidates(
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
