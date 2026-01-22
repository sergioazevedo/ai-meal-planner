package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
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
	textGen := llm.NewGroqClient(cfg)
	geminiClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	// 3. Initialize Ghost Client
	ghostClient := ghost.NewClient(cfg)

	// 4. Initialize Storage
	store, err := storage.NewRecipeStore(cfg.RecipeStoragePath)
	if err != nil {
		log.Fatalf("Failed to initialize recipe store: %v", err)
	}

	// 5. Initialize Services
	mealPlanner := planner.NewPlanner(store, textGen, geminiClient)
	recipeClipper := clipper.NewClipper(ghostClient, textGen)

	// 6. Initialize Telegram Bot
	bot, err := telegram.NewBot(cfg, mealPlanner, recipeClipper)
	if err != nil {
		log.Fatalf("Failed to initialize Telegram Bot: %v", err)
	}

	// 7. Start Server with Graceful Shutdown
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	bot.RegisterHandlers()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: nil,
	}

	go func() {
		log.Printf("Telegram Bot Server listening on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
