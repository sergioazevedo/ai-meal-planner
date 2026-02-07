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
	"ai-meal-planner/internal/storage"
)

// App holds the application's dependencies.
type App struct {
	ghostClient   ghost.Client
	textGen       llm.TextGenerator
	embedGen      llm.EmbeddingGenerator
	recipeStore   *storage.RecipeStore // Still needed for migration
	metricsStore  *metrics.Store
	mealPlanner   *planner.Planner
	recipeClipper *clipper.Clipper
	cfg           *config.Config

	// New database components
	db             *database.DB
	recipeRepo     *recipe.Repository
	vectorRepo     *llm.VectorRepository
	planRepo       *planner.PlanRepository
}

// NewApp creates and initializes a new App instance.
func NewApp(
	ghostClient ghost.Client,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	recipeStore *storage.RecipeStore, // Still needed for migration
	metricsStore *metrics.Store,
	mealPlanner *planner.Planner,
	recipeClipper *clipper.Clipper,
	cfg *config.Config,
	db *database.DB, // New parameter
	recipeRepo *recipe.Repository, // New parameter
	vectorRepo *llm.VectorRepository, // New parameter
	planRepo *planner.PlanRepository, // New parameter
) *App {
	return &App{
		ghostClient:   ghostClient,
		textGen:       textGen,
		embedGen:      embedGen,
		recipeStore:   recipeStore,
		metricsStore:  metricsStore,
		mealPlanner:   mealPlanner,
		recipeClipper: recipeClipper,
		cfg:           cfg,
		db:             db,
		recipeRepo:     recipeRepo,
		vectorRepo:     vectorRepo,
		planRepo:       planRepo,
	}
}

// IngestRecipes fetches and normalizes recipes from Ghost.
// This method will be updated to use the new RecipeRepository and VectorRepository
// instead of storage.RecipeStore for saving.
func (a *App) IngestRecipes(ctx context.Context) error {
	fmt.Println("Fetching and processing recipes...")

	posts, err := a.ghostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost.\n", len(posts))
	for _, post := range posts {
		// Existing check will need to be replaced with a database check later
		// For now, if we are still using file-based storage during ingest, this is fine.
		// If the migration utility has been run, this will need an update.
		if a.recipeStore.Exists(post.ID, post.UpdatedAt) {
			log.Printf("Recipe '%s' up-to-date in file storage. Skipping file save.", post.Title)
			// TODO: Add check for database existence here after migration
		} else {
			if err := a.recipeStore.RemoveStaleVersions(post.ID); err != nil {
				log.Printf("Warning: failed to clean up stale versions for '%s' in file storage: %v", post.Title, err)
			}
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

		if err != nil {
			log.Printf("Failed to normalize '%s': %v", post.Title, err)
			continue
		}

		// Save to new repositories
		if err := a.recipeRepo.Save(ctx, normalizedRecipe.Recipe); err != nil {
			log.Printf("Failed to save recipe '%s' to DB: %v", normalizedRecipe.Title, err)
			continue
		}
		if err := a.vectorRepo.Save(ctx, normalizedRecipe.ID, normalizedRecipe.Embedding); err != nil {
			log.Printf("Failed to save embedding for '%s' to DB: %v", normalizedRecipe.Title, err)
			continue
		}

		// Also save to file-based storage for backward compatibility during migration period
		// This will be removed once file-based storage is fully deprecated.
		if err := a.recipeStore.Save(normalizedRecipe); err != nil {
			log.Printf("Failed to save '%s' to file storage: %v", post.Title, err)
			continue
		}

		a.metricsStore.Record(metrics.ExecutionMetric{
			AgentName:        meta.AgentName,
			Model:            meta.Usage.Model,
			PromptTokens:     meta.Usage.PromptTokens,
			CompletionTokens: meta.Usage.CompletionTokens,
			LatencyMS:        meta.Latency.Milliseconds(),
		})


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

	plan, metas, err := a.mealPlanner.GeneratePlan(ctx, request, pCtx)
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
	planJSON, err := json.Marshal(plan)
	if err != nil {
		log.Printf("Warning: failed to marshal meal plan to JSON for saving: %v", err)
	} else {
		if err := a.planRepo.Save(ctx, "default_user", planJSON); err != nil { // TODO: Replace "default_user" with actual user ID
			log.Printf("Warning: failed to save meal plan to user memory: %v", err)
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

// MigrateRecipesFromFiles reads recipes from the file system and saves them to the database.
func (a *App) MigrateRecipesFromFiles(ctx context.Context) error {
	fmt.Println("Starting migration of recipes from file system to database...")

	// First, count existing recipes in DB to avoid unnecessary re-ingestion
	existingRecipes, err := a.recipeRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list existing recipes in DB: %w", err)
	}
	existingDBRecipeIDs := make(map[string]struct{})
	for _, rec := range existingRecipes {
		existingDBRecipeIDs[rec.ID] = struct{}{}
	}

	// List all recipes from the file-based store
	fileRecipes, err := a.recipeStore.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list recipes from file storage: %w", err)
	}

	fmt.Printf("Found %d recipes in file storage. %d already in DB.\n", len(fileRecipes), len(existingDBRecipeIDs))

	migratedCount := 0
	for _, normalizedRecipe := range fileRecipes {
		if _, exists := existingDBRecipeIDs[normalizedRecipe.ID]; exists {
			log.Printf("Recipe '%s' (ID: %s) already exists in DB. Skipping migration.", normalizedRecipe.Title, normalizedRecipe.ID)
			continue
		}

		log.Printf("Migrating recipe '%s' (ID: %s) from file to database...", normalizedRecipe.Title, normalizedRecipe.ID)

		// Save to RecipeRepository
		if err := a.recipeRepo.Save(ctx, normalizedRecipe.Recipe); err != nil {
			log.Printf("Failed to save recipe '%s' (ID: %s) to DB during migration: %v", normalizedRecipe.Title, normalizedRecipe.ID, err)
			continue
		}

		// Save embedding to VectorRepository
		if len(normalizedRecipe.Embedding) > 0 {
			if err := a.vectorRepo.Save(ctx, normalizedRecipe.ID, normalizedRecipe.Embedding); err != nil {
				log.Printf("Failed to save embedding for '%s' (ID: %s) to DB during migration: %v", normalizedRecipe.Title, normalizedRecipe.ID, err)
				continue
			}
		} else {
			log.Printf("Warning: Recipe '%s' (ID: %s) has no embedding. Skipping embedding save.", normalizedRecipe.Title, normalizedRecipe.ID)
		}

		migratedCount++
		log.Printf("Successfully migrated '%s'.", normalizedRecipe.Title)
	}

	fmt.Printf("Migration complete. Migrated %d new recipes to the database.\n", migratedCount)
	return nil
}
