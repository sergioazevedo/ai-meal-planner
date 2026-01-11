package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/config"
)

func main() {
	ctx := context.Background()

	cfg, err := config.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	application, cleanup, err := app.NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer cleanup()

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
}