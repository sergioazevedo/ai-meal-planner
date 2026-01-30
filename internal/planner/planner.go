package planner

import (
	"context"
	"fmt"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/storage"
)

// DayPlan represents the plan for a single day.
type DayPlan struct {
	Day         string `json:"day"`
	RecipeTitle string `json:"recipe_title"`
	PrepTime    string `json:"prep_time"`
	Note        string `json:"note"`
}

// MealPlan represents a full weekly meal plan.
type MealPlan struct {
	Plan         []DayPlan `json:"plan"`
	ShoppingList []string  `json:"shopping_list"`
}

// Planner handles the generation of meal plans.
type Planner struct {
	recipeStore *storage.RecipeStore
	textGen     llm.TextGenerator
	embedGen    llm.EmbeddingGenerator
}

// NewPlanner creates a new Planner instance.
func NewPlanner(store *storage.RecipeStore, textGen llm.TextGenerator, embedGen llm.EmbeddingGenerator) *Planner {
	return &Planner{
		recipeStore: store,
		textGen:     textGen,
		embedGen:    embedGen,
	}
}

// PlanningContext holds user-specific constraints for the meal plan.
type PlanningContext struct {
	Adults           int
	Children         int
	ChildrenAges     []int
	CookingFrequency int // Times per week they want to cook
}

// GeneratePlan creates a meal plan based on a user request.
func (p *Planner) GeneratePlan(ctx context.Context, userRequest string, pCtx PlanningContext) (*MealPlan, []llm.AgentMeta, error) {
	var metas []llm.AgentMeta

	// 1. Generate embedding for the user request to find relevant recipes
	queryEmbedding, err := p.embedGen.GenerateEmbedding(ctx, userRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate embedding for request: %w", err)
	}

	// 2. Retrieve top N relevant recipes
	// We fetch 9 recipes to give the LLM variety while staying within token limits
	recipes, err := p.recipeStore.FindSimilar(queryEmbedding, 9)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	if len(recipes) == 0 {
		return nil, nil, fmt.Errorf("no recipes found to create a plan")
	}

	// 4. Call Analyst agent to create a meal schedule
	analystResult, err := p.runAnalyst(ctx, userRequest, pCtx, recipes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate meal schedule: %w", err)
	}
	metas = append(metas, analystResult.Meta)

	// 5. Handover meal schedule to the chef to prempare
	// the MealPlan and the consolidate shooping list
	chefResult, err := p.runChef(ctx, analystResult.Proposal)
	if err != nil {
		return nil, metas, fmt.Errorf("failed to generate meal plan: %w", err)
	}
	metas = append(metas, chefResult.Meta)

	return chefResult.Plan, metas, nil
}
