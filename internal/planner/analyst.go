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

var submitMealProposalTool = llm.Tool{
	Name:        "submit_meal_proposal",
	Description: "Submit the final meal proposal. Use this tool ONLY when you have successfully gathered exactly 5 different recipes and organized them into the plan. This is your final action. Do not call this tool with empty or incomplete lists.",
	Parameters: llm.ToolParameters{
		Type: llm.ParameterTypeObject,
		Properties: map[string]llm.Property{
			"selected_recipes_audit": {
				Type:        llm.PropertyTypeArray,
				Description: "The titles of the 5 unique recipes selected.",
				Items: &llm.Property{
					Type: llm.PropertyTypeString,
				},
			},
			"planned_meals": {
				Type:        llm.PropertyTypeArray,
				Description: "The 9 planned meals schedule.",
				Items: &llm.Property{
					Type: llm.PropertyTypeObject,
					Properties: map[string]llm.Property{
						"day": {
							Type:        llm.PropertyTypeString,
							Description: "The day of the week (e.g., 'Monday').",
						},
						"action": {
							Type:        llm.PropertyTypeString,
							Description: "The action, either 'Cook' or 'Reuse'.",
						},
						"recipe_title": {
							Type:        llm.PropertyTypeString,
							Description: "The exact title of the recipe.",
						},
						"note": {
							Type:        llm.PropertyTypeString,
							Description: "Strategic reasoning for this meal.",
						},
					},
					Required: []string{"day", "action", "recipe_title", "note"},
				},
			},
		},
		Required: []string{"selected_recipes_audit", "planned_meals"},
	},
}

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

	raw := &rawLlmResult{}

	// 2. Setup Tool Handlers
	handlers := map[string]ToolHandler[[]value.Recipe]{
		searchRecipesSemanticTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeSemanticSearch(ctx, a.searcher, toolCall, recipesRecentlyUsed)
		},
		searchRecipesRandomTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			return HandleRecipeRandomSearch(ctx, a.searcher, toolCall, recipesRecentlyUsed)
		},
		submitMealProposalTool.Name: func(ctx context.Context, toolCall llm.ToolCall) (llm.Message, []value.Recipe, error) {
			b, err := json.Marshal(toolCall.Args)
			if err != nil {
				return llm.Message{}, nil, fmt.Errorf("failed to marshal terminal tool args: %w", err)
			}
			if err := json.Unmarshal(b, raw); err != nil {
				return llm.Message{}, nil, fmt.Errorf("failed to parse terminal tool args: %w", err)
			}
			return llm.Message{
				Role:       "tool",
				Content:    `{"status":"success"}`,
				ToolCallID: toolCall.ID,
			}, nil, ErrAgentFinished
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
			submitMealProposalTool,
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

	// 5. Parse JSON (Fallback to message content if terminal tool wasn't called)
	if len(raw.PlannedMeals) == 0 {
		cleanedJSON := llm.CleanJSON(resp.Message.Content)
		if err = json.Unmarshal([]byte(cleanedJSON), raw); err != nil {
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
