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
	for _, post := range posts {
		if !force {
			// Optimization: Check if recipe already exists with the same updatedAt
			exists, err := a.recipeRepo.Exists(ctx, post.ID, post.UpdatedAt)
			if err == nil && exists {
				log.Printf("Recipe '%s' is already up-to-date in DB. Skipping normalization.", post.Title)
				continue
			}
		}

		log.Printf("Normalizing '%s'...", post.Title)
		recipeWithEmbedding, meta, err := recipe.NormalizeHTML(
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

		// Use a transaction to ensure atomic save of recipe and embedding
		err = func() error {
			tx, err := a.db.SQL.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer tx.Rollback()

			// Use WithTx to get transaction-aware repos
			recipeRepoTx := a.recipeRepo.WithTx(tx)
			vectorRepoTx := a.vectorRepo.WithTx(tx)

			if err := recipeRepoTx.Save(ctx, recipeWithEmbedding.Recipe); err != nil {
				return fmt.Errorf("failed to save recipe: %w", err)
			}
			if err := vectorRepoTx.Save(ctx, recipeWithEmbedding.ID, recipeWithEmbedding.Embedding); err != nil {
				return fmt.Errorf("failed to save embedding: %w", err)
			}

			return tx.Commit()
		}()

		if err != nil {
			log.Printf("Failed to save recipe '%s' to DB: %v", recipeWithEmbedding.Title, err)
			continue
		}

		a.metricsStore.Record(metrics.ExecutionMetric{
			AgentName:        meta.AgentName,
			Model:            meta.Usage.Model,
			PromptTokens:     meta.Usage.PromptTokens,
			CompletionTokens: meta.Usage.CompletionTokens,
			LatencyMS:        meta.Latency.Milliseconds(),
		})

		log.Printf("Successfully processed '%s'.", recipeWithEmbedding.Title)

		// Wait 5 seconds to stay under Gemini Free Tier Rate Limits (15 RPM)
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
