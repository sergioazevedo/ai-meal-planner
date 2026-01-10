package main

import (
	"context"
	"log"

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

	if err := application.Run(ctx); err != nil {
		log.Fatalf("Application run failed: %v", err)
	}
}
