package app

import (
	"context"
	"fmt"
	"log"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

// App holds the application's dependencies.
type App struct {
	GhostClient ghost.Client
	LlmClient   llm.LLMClient
	RecipeStore *storage.RecipeStore
	Planner     *planner.Planner
}

// NewApp creates and initializes a new App instance.
func NewApp(ctx context.Context, cfg *config.Config) (*App, func(), error) {
	ghostClient := ghost.NewClient(cfg)

	llmClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create llm client: %w", err)
	}
	cleanup := func() {
		if err := llmClient.Close(); err != nil {
			log.Printf("Warning: failed to close llm client: %v", err)
		}
	}

	recipeStore, err := storage.NewRecipeStore("data/recipes")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create recipe store: %w", err)
	}

	mealPlanner := planner.NewPlanner(recipeStore, llmClient)

	return &App{
		GhostClient: ghostClient,
		LlmClient:   llmClient,
		RecipeStore: recipeStore,
		Planner:     mealPlanner,
	}, cleanup, nil
}

// IngestRecipes fetches and normalizes recipes from Ghost.
func (a *App) IngestRecipes(ctx context.Context) error {
	fmt.Println("Fetching and processing recipes...")

	posts, err := a.GhostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost.\n", len(posts))
	for _, post := range posts {
		if a.RecipeStore.Exists(post.ID, post.UpdatedAt) {
			log.Printf("Recipe '%s' up-to-date. Skipping.", post.Title)
			continue
		}

		if err := a.RecipeStore.RemoveStaleVersions(post.ID); err != nil {
			log.Printf("Warning: failed to clean up stale versions for '%s': %v", post.Title, err)
		}

		log.Printf("Normalizing '%s'வைக்...", post.Title)
		normalizedRecipe, err := recipe.NormalizeRecipeHTML(ctx, a.LlmClient, post)
		if err != nil {
			log.Printf("Failed to normalize '%s': %v", post.Title, err)
			continue
		}

		if err := a.RecipeStore.Save(post.ID, post.UpdatedAt, *normalizedRecipe); err != nil {
			log.Printf("Failed to save '%s': %v", post.Title, err)
			continue
		}
		log.Printf("Successfully processed '%s'.", normalizedRecipe.Title)
	}
	fmt.Println("Ingestion complete.")
	return nil
}

// GenerateMealPlan creates a meal plan based on user request and prints it.
func (a *App) GenerateMealPlan(ctx context.Context, request string) error {
	fmt.Printf("Generating meal plan for: \"%s\"...\n", request)

	plan, err := a.Planner.GeneratePlan(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	fmt.Println("\n=== WEEKLY MEAL PLAN ===")
	for _, dp := range plan.Plan {
		fmt.Printf("% -10s: %s\n", dp.Day, dp.RecipeTitle)
		if dp.Note != "" {
			fmt.Printf("            Note: %s\n", dp.Note)
		}
	}

	fmt.Println("\n=== SHOPPING LIST ===")
	for _, item := range plan.ShoppingList {
		fmt.Printf("- %s\n", item)
	}

	fmt.Printf("\nPrep Estimate: %s\n", plan.TotalPrep)
	return nil
}