package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/storage"
)

func main() {
	ctx := context.Background()

	cfg, err := config.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ghostClient := ghost.NewClient(cfg)

	geminiClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Gemini client: %v", err)
	}
	defer geminiClient.Close()

	groqClient := llm.NewGroqClient(cfg)

	recipeStore, err := storage.NewRecipeStore(cfg.RecipeStoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize recipe store: %v", err)
	}

	metricsStore, err := metrics.NewStore(cfg.MetricsDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize metrics store: %v", err)
	}
	defer metricsStore.Close()

	mealPlanner := planner.NewPlanner(recipeStore, groqClient, geminiClient)
	recipeClipper := clipper.NewClipper(ghostClient, groqClient)

	application := app.NewApp(ghostClient, groqClient, geminiClient, recipeStore, metricsStore, mealPlanner, recipeClipper, cfg)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ingest":
		if err := application.IngestRecipes(ctx); err != nil {
			log.Fatalf("Ingestion failed: %v", err)
		}
	case "plan":
		planCmd := flag.NewFlagSet("plan", flag.ExitOnError)
		request := planCmd.String("request", "", "User request for the meal plan (e.g., 'healthy dinners')")
		planCmd.Parse(os.Args[2:])

		if *request == "" {
			fmt.Println("Error: -request flag is required for the plan command.")
			planCmd.Usage()
			os.Exit(1)
		}

		if err := application.GenerateMealPlan(ctx, *request); err != nil {
			log.Fatalf("Planning failed: %v", err)
		}
	case "metrics-cleanup":
		cleanupCmd := flag.NewFlagSet("metrics-cleanup", flag.ExitOnError)
		days := cleanupCmd.Int("days", 30, "Keep records for the last N days")
		cleanupCmd.Parse(os.Args[2:])

		mStore, err := metrics.NewStore(cfg.MetricsDBPath)
		if err != nil {
			log.Fatalf("Failed to open metrics store: %v", err)
		}
		defer mStore.Close()

		affected, err := mStore.Cleanup(*days)
		if err != nil {
			log.Fatalf("Cleanup failed: %v", err)
		}
		fmt.Printf("Successfully removed %d old metric records.\n", affected)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: ai-meal-planner <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  ingest             Fetch and normalize recipes from Ghost")
	fmt.Println("  plan -request \"...\" Generate a weekly meal plan")
	fmt.Println("  metrics-cleanup    Remove old metric records")
}
