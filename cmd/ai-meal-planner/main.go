package main

import (
	"fmt"
	"log"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
)

func main() {
	fmt.Println("AI Meal Planner application starting...")

	cfg, err := config.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	ghostClient := ghost.NewClient(cfg)

	recipes, err := ghostClient.FetchRecipes()
	if err != nil {
		log.Fatalf("Failed to fetch recipes: %v", err)
	}

	fmt.Printf("Successfully fetched %d recipes:\n", len(recipes))
	for _, recipe := range recipes {
		fmt.Printf("- %s\n", recipe.Title)
	}
}
