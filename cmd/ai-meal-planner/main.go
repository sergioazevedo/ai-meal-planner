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
	"ai-meal-planner/internal/database" // New import
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe" // New import
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

	// Initialize the new SQLite database
	db, err := database.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize new repositories
	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	planRepo := planner.NewPlanRepository(db.SQL)

	metricsStore := metrics.NewStore(db.SQL)
	defer metricsStore.Close()

	mealPlanner := planner.NewPlanner(recipeRepo, vectorRepo, planRepo, groqClient, geminiClient)
	recipeClipper := clipper.NewClipper(ghostClient, groqClient)

	application := app.NewApp(
		ghostClient,
		groqClient,   // textGen
		geminiClient, // embedGen
		metricsStore,
		mealPlanner,
		recipeClipper,
		cfg,
		db,         // Pass new DB
		recipeRepo, // Pass new RecipeRepo
		vectorRepo, // Pass new VectorRepo
		planRepo,   // Pass new PlanRepo
	)

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ingest":
		ingestCmd := flag.NewFlagSet("ingest", flag.ExitOnError)
		force := ingestCmd.Bool("force", false, "Force re-ingestion even if recipe is up-to-date")
		ingestCmd.Parse(os.Args[2:])

		if err := application.IngestRecipes(ctx, *force); err != nil {
			log.Fatalf("Ingestion failed: %v", err)
		}
	case "plan":
		planCmd := flag.NewFlagSet("plan", flag.ExitOnError)
		request := planCmd.String("request", "", "What would you like to eat?")
		userID := planCmd.String("user", "cli_user", "User identifier for memory")
		planCmd.Parse(os.Args[2:])

		if *request == "" {
			fmt.Println("Error: -request is required")
			planCmd.Usage()
			os.Exit(1)
		}

		if err := application.GenerateMealPlan(ctx, *userID, *request); err != nil {
			log.Fatalf("Meal planning failed: %v", err)
		}
	case "metrics-cleanup":
		cleanupCmd := flag.NewFlagSet("metrics-cleanup", flag.ExitOnError)
		days := cleanupCmd.Int("days", 30, "Keep records for the last N days")
		cleanupCmd.Parse(os.Args[2:])

		affected, err := metricsStore.Cleanup(*days)
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
	fmt.Println("  metrics-cleanup    Remove old metric records")
}
