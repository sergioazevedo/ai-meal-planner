package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/database" // New import
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
)

// App holds the application's dependencies.
type App struct {
	ghostClient   ghost.Client
	textGen       llm.TextGenerator
	embedGen      llm.EmbeddingGenerator
	metricsStore  *metrics.Store
	mealPlanner   *planner.Planner
	recipeClipper *clipper.Clipper
	cfg           *config.Config

	// New database components
	db         *database.DB
	recipeRepo *recipe.Repository
	vectorRepo *llm.VectorRepository
	planRepo   *planner.PlanRepository

	extractor *recipe.Extractor // New Extractor instance
}

// NewApp creates and initializes a new App instance.
func NewApp(
	ghostClient ghost.Client,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	metricsStore *metrics.Store,
	mealPlanner *planner.Planner,
	recipeClipper *clipper.Clipper,
	cfg *config.Config,
	db *database.DB,
	recipeRepo *recipe.Repository,
	vectorRepo *llm.VectorRepository,
	planRepo *planner.PlanRepository,
) *App {
	return &App{
		ghostClient:   ghostClient,
		textGen:       textGen,
		embedGen:      embedGen,
		metricsStore:  metricsStore,
		mealPlanner:   mealPlanner,
		recipeClipper: recipeClipper,
		cfg:           cfg,
		db:            db,
		recipeRepo:    recipeRepo,
		vectorRepo:    vectorRepo,
		planRepo:      planRepo,
		extractor:     recipe.NewExtractor(textGen, embedGen, vectorRepo), // Initialize Extractor
	}
}

// IngestRecipes fetches and normalizes recipes from Ghost.
func (a *App) IngestRecipes(ctx context.Context, force bool) error {
	fmt.Println("Fetching and processing recipes...")

	posts, err := a.ghostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost.\n", len(posts))

	// Ingest recipes within the loop, inlining the previous processSingleRecipe logic
	for _, post := range posts {
		// The database-level UPSERT now handles conditional updates based on the `updated_at` timestamp.
		// The `force` flag ensures normalization always runs, but the DB handles the save logic.

		log.Printf("Normalizing '%s'...", post.Title)

		// Inlined logic from processSingleRecipe
		if err := ProcessAndSaveRecipe(
			ctx,
			a.extractor, // Pass the extractor
			a.recipeRepo,
			a.metricsStore,
			post,
		); err != nil {
			log.Printf("Failed to process recipe '%s': %v", post.Title, err)
		} else {
			log.Printf("Successfully processed '%s'.", post.Title)
		}

		// Wait 5 seconds to stay under Rate Limits (Gemini Free Tier: 15 RPM, Groq: various)
		// We sleep even on failure to ensure we don't hammer the API after a 429 error.
		time.Sleep(5 * time.Second)
	}
	fmt.Println("Ingestion complete.")
	return nil
}

// GenerateMealPlan creates a meal plan based on user request and prints it.
func (a *App) GenerateMealPlan(ctx context.Context, userID string, request string) error {
	fmt.Printf("Generating meal plan for: \"%s\"...\n", request)

	// Use defaults from config
	pCtx := planner.PlanningContext{
		Adults:           a.cfg.DefaultAdults,
		Children:         a.cfg.DefaultChildren,
		ChildrenAges:     a.cfg.DefaultChildrenAges,
		CookingFrequency: a.cfg.DefaultCookingFrequency,
	}

	targetWeek := planner.GetNextMonday(time.Now())
	plan, metas, err := a.mealPlanner.GeneratePlan(ctx, userID, request, pCtx, targetWeek)
	if err != nil {
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Record metrics for each agent execution
	for _, meta := range metas {
		if err := a.metricsStore.Record(metrics.MapUsage(meta.AgentName, meta.Usage, meta.Latency)); err != nil {
			log.Printf("Warning: failed to record metrics for %s: %v", meta.AgentName, err)
		}
	}

	// Save the generated meal plan to user memory
	if err := a.planRepo.Save(ctx, userID, plan); err != nil {
		log.Printf("Warning: failed to save meal plan to user memory: %v", err)
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
