package main

import (
	"context"
	"log"
	"os"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/storage"
	"ai-meal-planner/internal/telegram"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	// 2. Initialize Infrastructure (LLMs)
	// Text Generation (Groq)
	// NewGroqClient takes cfg and returns the interface (no error return)
	textGen := llm.NewGroqClient(cfg)

	// Embeddings (Gemini)
	geminiClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 3. Initialize Storage
	// The bot needs to know about existing recipes to plan with them.
	store, err := storage.NewRecipeStore("recipes_data")
	if err != nil {
		log.Fatalf("Failed to initialize recipe store: %v", err)
	}
	log.Println("Recipe store initialized (reads from 'recipes_data' directory).")

	// 4. Initialize Planner
	mealPlanner := planner.NewPlanner(store, textGen, geminiClient)

	// 5. Initialize Telegram Bot
	bot, err := telegram.NewBot(cfg, mealPlanner)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram Bot: %v", err)
	}

	// 6. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("Starting Telegram Bot...")
	if err := bot.StartServer(port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}