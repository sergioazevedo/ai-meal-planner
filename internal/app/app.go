package app

import (
	"context"
	"fmt"
	"log"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"
)

// App holds the application's dependencies.
type App struct {
	GhostClient ghost.Client
	LlmClient   llm.LLMClient
	RecipeStore *storage.RecipeStore
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

	return &App{
		GhostClient: ghostClient,
		LlmClient:   llmClient,
		RecipeStore: recipeStore,
	}, cleanup, nil
}

// Run executes the main application logic.
func (a *App) Run(ctx context.Context) error {
	fmt.Println("Fetching and processing recipes...")

	posts, err := a.GhostClient.FetchRecipes()
	if err != nil {
		return fmt.Errorf("failed to fetch recipes from ghost: %w", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost.\n", len(posts))
	for _, post := range posts {
		// Check if the specific version already exists
		if a.RecipeStore.Exists(post.ID, post.UpdatedAt) {
			log.Printf("Recipe '%s' (ID: %s) up-to-date (Version: %s). Skipping.", post.Title, post.ID, post.UpdatedAt)
			continue
		}

		// New version found or recipe not present. Clean up old versions first.
		if err := a.RecipeStore.RemoveStaleVersions(post.ID); err != nil {
			log.Printf("Warning: failed to clean up stale versions for recipe '%s': %v", post.Title, err)
			// Proceeding anyway as this is non-fatal, though ideally we want a clean state
		}

		log.Printf("Normalizing recipe '%s' (ID: %s)...", post.Title, post.ID)
		normalizedRecipe, err := recipe.NormalizeRecipeHTML(ctx, a.LlmClient, post)
		if err != nil {
			log.Printf("Failed to normalize recipe '%s': %v", post.Title, err)
			continue
		}

		// Set the version metadata
		normalizedRecipe.SourceUpdatedAt = post.UpdatedAt

		if err := a.RecipeStore.Save(post.ID, post.UpdatedAt, *normalizedRecipe); err != nil {
			log.Printf("Failed to save normalized recipe '%s': %v", post.Title, err)
			continue
		}
		log.Printf("Successfully normalized and saved recipe '%s'.", normalizedRecipe.Title)
	}
	fmt.Println("Processing complete.")
	return nil
}
