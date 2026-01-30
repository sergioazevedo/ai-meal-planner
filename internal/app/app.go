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
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

// App holds the application's dependencies.
type App struct {
	ghostClient  ghost.Client
	textGen      llm.TextGenerator
	embedGen     llm.EmbeddingGenerator
	recipeStore  *storage.RecipeStore
	metricsStore *metrics.Store
	planner      *planner.Planner
	clipper      *clipper.Clipper
	cfg          *config.Config
}

// NewApp creates and initializes a new App instance.
func NewApp(
	ghostClient ghost.Client,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	recipeStore *storage.RecipeStore,
	metricsStore *metrics.Store,
	mealPlanner *planner.Planner,
	recipeClipper *clipper.Clipper,
	cfg *config.Config,
) *App {
	return &App{
		ghostClient:  ghostClient,
		textGen:      textGen,
		embedGen:     embedGen,
		recipeStore:  recipeStore,
		metricsStore: metricsStore,
		planner:      mealPlanner,
		clipper:      recipeClipper,
		cfg:          cfg,
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
		normalizedRecipe, meta, err := recipe.NormalizeHTML(
			ctx,
			a.textGen,
			a.embedGen,
			recipe.PostData{
				ID:        post.ID,
				Title:     post.Title,
				UpdatedAt: post.UpdatedAt,
				HTML:      post.HTML,
			},
		)

		a.metricsStore.Record(metrics.ExecutionMetric{
			AgentName:        meta.AgentName,
			Model:            meta.Usage.Model,
			PromptTokens:     meta.Usage.PromptTokens,
			CompletionTokens: meta.Usage.CompletionTokens,
			LatencyMS:        meta.Latency.Milliseconds(),
		})

		if err != nil {
			log.Printf("Failed to normalize '%s': %v", post.Title, err)
			continue
		}

		if err := a.recipeStore.Save(normalizedRecipe); err != nil {
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

	plan, metas, err := a.planner.GeneratePlan(ctx, request, pCtx)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Record metrics for each agent execution
	for _, meta := range metas {
		if err := a.metricsStore.Record(metrics.MapUsage(meta.AgentName, meta.Usage, meta.Latency)); err != nil {
			log.Printf("Warning: failed to record metrics for %s: %v", meta.AgentName, err)
		}
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
