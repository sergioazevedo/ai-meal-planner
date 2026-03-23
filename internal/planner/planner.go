package planner

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/shared"
	// Removed "ai-meal-planner/internal/storage" as it's no longer directly used by Planner
)

// Planner handles the generation of meal plans.
type Planner struct {
	recipeRepo        *recipe.Repository
	vectorRepo        *llm.VectorRepository
	planRepo          *PlanRepository
	analystGenerator  llm.TextGenerator // High-reasoning model (e.g., 70B)
	chefGenerator     llm.TextGenerator // High-throughput model (e.g., 8B)
	reviewerGenerator llm.TextGenerator // High-reasoning model for plan revision
	embedGen          llm.EmbeddingGenerator
}

// NewPlanner creates a new Planner instance with separate generators for different agent roles.
func NewPlanner(
	recipeRepo *recipe.Repository,
	vectorRepo *llm.VectorRepository,
	planRepo *PlanRepository,
	analystGen llm.TextGenerator,
	chefGen llm.TextGenerator,
	reviewerGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
) *Planner {
	return &Planner{
		recipeRepo:        recipeRepo,
		vectorRepo:        vectorRepo,
		planRepo:          planRepo,
		analystGenerator:  analystGen,
		chefGenerator:     chefGen,
		reviewerGenerator: reviewerGen,
		embedGen:          embedGen,
	}
}

// GetNextMonday returns the time.Time for the next upcoming Monday at 00:00:00.
// If today is Monday, it returns next week's Monday.
func GetNextMonday(t time.Time) time.Time {
	daysUntilMonday := int(time.Monday - t.Weekday())
	if daysUntilMonday <= 0 {
		daysUntilMonday += 7
	}
	nextMonday := t.AddDate(0, 0, daysUntilMonday)
	return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 0, 0, 0, 0, nextMonday.Location())
}

// PlanningContext holds user-specific constraints for the meal plan.
type PlanningContext struct {
	Adults           int
	Children         int
	ChildrenAges     []int
	CookingFrequency int // Times per week they want to cook
}

func (p *Planner) receiptIDsRecentlyUsed(
	ctx context.Context,
	userID string,
	targetWeek time.Time,
) []string {
	result := []string{}
	// Check the last 3 plans
	recentPlans, err := p.planRepo.ListRecentByUserID(ctx, userID, 3)
	if err == nil {
		for _, plan := range recentPlans {
			// Skip the plan we are currently redoing/replacing
			if plan.WeekStart.Equal(targetWeek) {
				continue
			}
			for _, day := range plan.Plan {
				if day.RecipeID != "" {
					result = append(result, day.RecipeID)
				}
			}
		}
	}

	return result
}

// GeneratePlan creates a meal plan based on a user request.
func (p *Planner) GeneratePlan(ctx context.Context, userID string, userRequest string, pCtx PlanningContext, targetWeek time.Time) (*MealPlan, []shared.AgentMeta, error) {
	var metas []shared.AgentMeta

	// 0. Fetch recent history to avoid repetition
	excludeIDs := p.receiptIDsRecentlyUsed(ctx, userID, targetWeek)

	// 1. Fetch intial set of recipes
	recipeSelection, err := p.getRecipeCandidates(ctx, userRequest, excludeIDs)
	if err != nil {
		return nil, nil, err
	}

	// 1. Call Analyst agent to create a meal schedule
	analystResult, err := p.runAnalyst(
		ctx,
		userRequest,
		pCtx,
		recipeSelection,
		excludeIDs,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate meal schedule: %w", err)
	}
	metas = append(metas, analystResult.Meta)

	// 5. Handover meal schedule to the chef to prempare
	// the MealPlan and the consolidate shooping list
	chefResult, err := p.runChef(ctx, analystResult.Proposal, targetWeek)
	if err != nil {
		return nil, metas, fmt.Errorf("failed to generate meal plan: %w", err)
	}
	chefResult.Plan.OriginalRequest = userRequest
	metas = append(metas, chefResult.Meta)

	return chefResult.Plan, metas, nil
}

// GenerateShoppingList generates a shopping list for an existing meal plan
// This is used when confirming a draft plan or after adjustments
func (p *Planner) GenerateShoppingList(ctx context.Context, plan *MealPlan, pCtx PlanningContext) ([]string, error) {
	// 1. Extract unique recipe IDs from the plan
	recipeIDMap := make(map[string]bool)
	var plannedMeals []PlannedMeal

	for _, day := range plan.Plan {
		if day.RecipeID != "" {
			recipeIDMap[day.RecipeID] = true

			// Determine action based on day title
			action := MealActionCook
			if strings.Contains(strings.ToLower(day.RecipeTitle), "leftover") ||
				strings.Contains(strings.ToLower(day.RecipeTitle), "reuse") {
				action = MealActionLeftOvers
			}

			plannedMeals = append(plannedMeals, PlannedMeal{
				Day:         day.Day,
				RecipeID:    day.RecipeID,
				Action:      action,
				RecipeTitle: day.RecipeTitle,
				Note:        day.Note,
			})
		}
	}

	// 2. Fetch all recipes
	var recipeIDs []string
	for id := range recipeIDMap {
		recipeIDs = append(recipeIDs, id)
	}

	recipes, err := p.recipeRepo.GetByIds(ctx, recipeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recipes: %w", err)
	}

	// 3. Create a MealProposal
	proposal := &MealProposal{
		PlannedMeals: plannedMeals,
		Recipes:      recipes,
		Adults:       pCtx.Adults,
		Children:     pCtx.Children,
		ChildrenAges: pCtx.ChildrenAges,
	}

	// 4. Call Chef to generate the shopping list
	chefResult, err := p.runChef(ctx, proposal, plan.WeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to generate shopping list: %w", err)
	}

	return chefResult.Plan.ShoppingList, nil
}

// RevisePlan revises an existing meal plan based on user feedback.
func (p *Planner) RevisePlan(
	ctx context.Context,
	currentPlan *MealPlan,
	originalRequest string,
	feedback string,
	pCtx PlanningContext,
) (PlanReviewerResult, error) {
	// Find relevant recipes based on feedback
	recipes, err := p.getRecipeCandidates(ctx, feedback, nil)
	if err != nil {
		return PlanReviewerResult{}, fmt.Errorf("failed to find recipe candidates: %w", err)
	}

	log.Printf("PlanReviewer will choose from %d available recipes", len(recipes))

	// Run the reviewer agent
	return p.RunPlanReviewer(ctx, currentPlan, originalRequest, feedback, pCtx, recipes)
}

// getRecipeCandidates retrieves recipe candidates based on a query string.
func (p *Planner) getRecipeCandidates(ctx context.Context, query string, excludeIDs []string) ([]recipe.Recipe, error) {
	queryEmbedding, err := p.embedGen.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding for request: %w", err)
	}

	recipeIds, err := p.vectorRepo.FindSimilar(ctx, queryEmbedding, 10, excludeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
	}

	if len(recipeIds) < 5 {
		log.Printf("Warning: Recipe pool exhausted for query '%s'. Dropping exclusions.", query)
		recipeIds, err = p.vectorRepo.FindSimilar(ctx, queryEmbedding, 10, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve similar recipes: %w", err)
		}
	}

	recipes, err := p.recipeRepo.GetByIds(ctx, recipeIds)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve recipes: %w", err)
	}

	return recipes, nil
}
