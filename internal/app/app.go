package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

// App holds the application's dependencies.
type App struct {
	ghostClient ghost.Client
	textGen     llm.TextGenerator
	embedGen    llm.EmbeddingGenerator
	recipeStore *storage.RecipeStore
	planner     *planner.Planner
	clipper     *clipper.Clipper
	cfg         *config.Config
}

// NewApp creates and initializes a new App instance.
func NewApp(
	ghostClient ghost.Client,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	recipeStore *storage.RecipeStore,
	mealPlanner *planner.Planner,
	recipeClipper *clipper.Clipper,
	cfg *config.Config,
) *App {
	return &App{
		ghostClient: ghostClient,
		textGen:     textGen,
		embedGen:    embedGen,
		recipeStore: recipeStore,
		planner:     mealPlanner,
		clipper:     recipeClipper,
		cfg:         cfg,
	}
}

// IngestRecipes fetches and normalizes recipes from Ghost.
func (a *App) IngestRecipes(ctx context.Context) error {
	fmt.Println("Fetching and processing recipes...")

	posts, err := a.ghostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost.\n", len(posts))
	for _, post := range posts {
		if a.recipeStore.Exists(post.ID, post.UpdatedAt) {
			log.Printf("Recipe '%s' up-to-date. Skipping.", post.Title)
			continue
		}

		if err := a.recipeStore.RemoveStaleVersions(post.ID); err != nil {
			log.Printf("Warning: failed to clean up stale versions for '%s': %v", post.Title, err)
		}

		log.Printf("Normalizing '%s'...", post.Title)
		normalizedRecipe, err := recipe.NormalizeRecipeHTML(ctx, a.textGen, a.embedGen, post)
		if err != nil {
			log.Printf("Failed to normalize '%s': %v", post.Title, err)
			continue
		}

		if err := a.recipeStore.Save(post.ID, post.UpdatedAt, *normalizedRecipe); err != nil {
			log.Printf("Failed to save '%s': %v", post.Title, err)
			continue
		}
		log.Printf("Successfully processed '%s'.", normalizedRecipe.Title)

		// Wait 5 seconds to stay under Gemini Free Tier Rate Limits (15 RPM)
		time.Sleep(5 * time.Second)
	}
	fmt.Println("Ingestion complete.")
	return nil
}

// GenerateMealPlan creates a meal plan based on user request and prints it.
func (a *App) GenerateMealPlan(ctx context.Context, request string) error {
	fmt.Printf("Generating meal plan for: \"%s\"...\n", request)

	// Use defaults from config
	pCtx := planner.PlanningContext{
		Adults:           a.cfg.DefaultAdults,
		Children:         a.cfg.DefaultChildren,
		ChildrenAges:     a.cfg.DefaultChildrenAges,
		CookingFrequency: a.cfg.DefaultCookingFrequency,
	}

	plan, _, err := a.planner.GeneratePlan(ctx, request, pCtx)
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

	return nil
}
