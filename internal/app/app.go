package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"ai-meal-planner/internal/audit"
	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/value"
)

const defaultBulkRetagDelay = 5 * time.Second

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
	auditRepo  *audit.AuditRepository

	extractor  *recipe.Extractor // New Extractor instance
	tagger     *recipe.Tagger
	retagDelay time.Duration
}

// NewApp creates and initializes a new App instance.
func NewApp(
	ghostClient ghost.Client,
	textGen llm.TextGenerator,
	tagGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	metricsStore *metrics.Store,
	mealPlanner *planner.Planner,
	recipeClipper *clipper.Clipper,
	cfg *config.Config,
	db *database.DB,
	recipeRepo *recipe.Repository,
	vectorRepo *llm.VectorRepository,
	planRepo *planner.PlanRepository,
	auditRepo *audit.AuditRepository,
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
		auditRepo:     auditRepo,
		extractor:     recipe.NewExtractor(textGen, embedGen, vectorRepo), // Initialize Extractor
		tagger:        recipe.NewTagger(tagGen),
		retagDelay:    defaultBulkRetagDelay,
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

	// Track fetched IDs for cleanup
	fetchedIDs := make(map[string]struct{})

	// Ingest recipes within the loop, inlining the previous processSingleRecipe logic
	for _, post := range posts {
		fetchedIDs[post.ID] = struct{}{}

		// The database-level UPSERT now handles conditional updates based on the `updated_at` timestamp.
		// The `force` flag ensures normalization always runs, but the DB handles the save logic.

		log.Printf("Normalizing '%s'...", post.Title)

		// Inlined logic from processSingleRecipe
		if err := ProcessAndSaveRecipe(
			ctx,
			a.extractor, // Pass the extractor
			a.tagger,
			a.recipeRepo,
			a.metricsStore,
			post,
			force,
		); err != nil {
			log.Printf("Failed to process recipe '%s': %v", post.Title, err)
		} else {
			log.Printf("Successfully processed '%s'.", post.Title)
		}
	}

	// Cleanup phase: remove recipes that are no longer in Ghost
	fmt.Println("Cleaning up orphaned recipes...")
	localRecipes, err := a.recipeRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list local recipes for cleanup: %w", err)
	}

	for _, rec := range localRecipes {
		if _, exists := fetchedIDs[rec.ID]; !exists {
			log.Printf("Removing orphaned recipe '%s' (ID: %s)...", rec.Title, rec.ID)
			if err := a.recipeRepo.Delete(ctx, rec.ID); err != nil {
				log.Printf("Failed to delete orphaned recipe %s: %v", rec.ID, err)
			}
		}
	}

	fmt.Println("Ingestion complete.")
	return nil
}

// IngestRecipeByID fetches and re-processes a single recipe from Ghost by its ID.
func (a *App) IngestRecipeByID(ctx context.Context, id string) error {
	fmt.Printf("Fetching and processing recipe ID: %s...\n", id)

	post, err := a.ghostClient.FetchRecipeByID(id)
	if err != nil {
		return fmt.Errorf("failed to fetch recipe %s from ghost: %w", id, err)
	}

	log.Printf("Normalizing '%s'...", post.Title)

	if err := ProcessAndSaveRecipe(
		ctx,
		a.extractor,
		a.tagger,
		a.recipeRepo,
		a.metricsStore,
		*post,
		true, // Force re-ingestion for manual single ID requests
	); err != nil {
		return fmt.Errorf("failed to process recipe '%s': %w", post.Title, err)
	}

	fmt.Printf("Successfully re-ingested '%s'.\n", post.Title)
	return nil
}

// RetagRecipeByID regenerates only a recipe's bilingual tags and its dependent embedding.
func (a *App) RetagRecipeByID(ctx context.Context, id string) error {
	post, err := a.ghostClient.FetchRecipeByID(id)
	if err != nil {
		return fmt.Errorf("failed to fetch recipe %s from ghost: %w", id, err)
	}
	if post == nil {
		return fmt.Errorf("ghost returned no recipe for %s", id)
	}

	rec, err := a.recipeRepo.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load recipe %s: %w", id, err)
	}
	return a.retagRecipe(ctx, rec, *post)
}

// RetagAllRecipes regenerates tags for every normalized recipe still present in Ghost.
func (a *App) RetagAllRecipes(ctx context.Context) error {
	posts, err := a.ghostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	processed := 0
	failed := 0
	for i, post := range posts {
		rec, err := a.recipeRepo.Get(ctx, post.ID)
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Skipping tag regeneration for %q: recipe is not normalized locally", post.Title)
			continue
		}
		if err != nil {
			failed++
			log.Printf("Failed to load recipe %q for retagging: %v", post.Title, err)
			continue
		}

		if err := a.retagRecipe(ctx, rec, post); err != nil {
			failed++
			log.Printf("Failed to retag recipe %q: %v", post.Title, err)
		} else {
			processed++
		}

		if a.retagDelay > 0 && i < len(posts)-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.retagDelay):
			}
		}
	}

	fmt.Printf("Retagged %d recipes.\n", processed)
	if failed > 0 {
		return fmt.Errorf("failed to retag %d recipes", failed)
	}
	return nil
}

func (a *App) retagRecipe(ctx context.Context, rec value.Recipe, post ghost.Post) error {
	result, err := a.tagger.Run(ctx, rec, ghostTagNames(post.Tags))
	if err != nil {
		return fmt.Errorf("failed to retag recipe %q: %w", rec.Title, err)
	}
	rec.Tags = result.Tags

	if err := a.recipeRepo.UpdateTags(ctx, rec); err != nil {
		return fmt.Errorf("failed to save retagged recipe %q: %w", rec.Title, err)
	}
	if err := a.metricsStore.RecordMeta(result.Meta); err != nil {
		return fmt.Errorf("failed to record tagger metrics: %w", err)
	}

	_, embeddingMeta, err := a.extractor.ProcessAndSaveEmbedding(ctx, rec, true)
	if err != nil {
		return fmt.Errorf("failed to refresh embedding for %q: %w", rec.Title, err)
	}
	if err := a.metricsStore.RecordMeta(embeddingMeta); err != nil {
		return fmt.Errorf("failed to record embedding metrics: %w", err)
	}

	fmt.Printf("Successfully retagged '%s'.\n", rec.Title)
	return nil
}

func ghostTagNames(tags []ghost.Tag) []string {
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
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
		if err := a.metricsStore.Record(metrics.MapUsage(meta)); err != nil {
			log.Printf("Warning: failed to record metrics for %s: %v", meta.AgentName, err)
		}
	}

	// Save the generated meal plan to user memory
	if _, err := a.planRepo.Save(ctx, userID, plan); err != nil {
		log.Printf("Warning: failed to save meal plan to user memory: %v", err)
	}

	fmt.Println("\n=== WEEKLY MEAL PLAN ===")
	for _, dp := range plan.Plan {
		fmt.Printf("% -10s: %s\n", dp.Day, dp.RecipeTitle)
		if len(dp.SideDishes) > 0 {
			fmt.Printf("            Side dishes: %s\n", strings.Join(dp.SideDishes, ", "))
		}
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

// GetShoppingListForPlan returns the shopping list for a specific plan ID.
func (a *App) GetShoppingListForPlan(ctx context.Context, planID int64) ([]string, error) {
	plan, err := a.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan: %w", err)
	}

	pCtx := planner.PlanningContext{
		Adults:           a.cfg.DefaultAdults,
		Children:         a.cfg.DefaultChildren,
		ChildrenAges:     a.cfg.DefaultChildrenAges,
		CookingFrequency: a.cfg.DefaultCookingFrequency,
	}

	return a.mealPlanner.GenerateShoppingList(ctx, plan, pCtx)
}
