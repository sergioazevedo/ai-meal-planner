package main

import (
	"context"
	"fmt"
	"log"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
)

func main() {
	ctx := context.Background()

	fmt.Println("AI Meal Planner application starting...")

	cfg, err := config.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ghostClient := ghost.NewClient(cfg)

	geminiClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	posts, err := ghostClient.FetchRecipes()
	if err != nil {
		log.Fatalf("Failed to fetch recipes: %v", err)
	}

	fmt.Printf("Successfully fetched %d recipe posts from Ghost:\n", len(posts))
	for _, post := range posts {
		normalizedRecipe, err := recipe.NormalizeRecipeHTML(ctx, geminiClient, post)
		if err != nil {
			log.Printf("Failed to normalize recipe '%s': %v", post.Title, err)
			continue
		}
		fmt.Printf("--- Normalized Recipe: %s ---\n", normalizedRecipe.Title)
		fmt.Printf("Ingredients: %v...\n", normalizedRecipe.Ingredients)
		// For brevity, not printing all instructions/tags here.
	}
}
