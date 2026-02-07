package planner

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/shared"
	// Removed "ai-meal-planner/internal/storage" as it's no longer directly used by Planner
)

// Planner handles the generation of meal plans.
type Planner struct {
	recipeRepo *recipe.Repository
	vectorRepo *llm.VectorRepository
	textGen    llm.TextGenerator
	embedGen   llm.EmbeddingGenerator
}

// NewPlanner creates a new Planner instance.
func NewPlanner(recipeRepo *recipe.Repository, vectorRepo *llm.VectorRepository, textGen llm.TextGenerator, embedGen llm.EmbeddingGenerator) *Planner {
	return &Planner{
		recipeRepo: recipeRepo,
		vectorRepo: vectorRepo,
		textGen:    textGen,
		embedGen:   embedGen,
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
func (p *Planner) GeneratePlan(ctx context.Context, userRequest string, pCtx PlanningContext) (*MealPlan, []shared.AgentMeta, error) {
	var metas []shared.AgentMeta
	var recipes []recipe.Recipe

	// 1. Decide retrieval strategy based on total recipe count
	count, err := p.recipeRepo.Count(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to count recipes: %w", err)
	}

	if count <= 20 {
		// For small pools, give everything to the Analyst to maximize variety
		recipes, err = p.recipeRepo.List(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list all recipes: %w", err)
		}
		// Shuffle to avoid positional bias in the LLM
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		r.Shuffle(len(recipes), func(i, j int) {
			recipes[i], recipes[j] = recipes[j], recipes[i]
		})
	} else {
		// 2. For larger pools, use embedding search to find top 9 relevant recipes
		queryEmbedding, err := p.embedGen.GenerateEmbedding(ctx, userRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate embedding for request: %w", err)
		}

		recipeIds, err := p.vectorRepo.FindSimilar(ctx, queryEmbedding, 20)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
		}
		recipes, err = p.recipeRepo.GetByIds(ctx, recipeIds)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to retrieve recipes: %w", err)
		}
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
