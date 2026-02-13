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
	"ai-meal-planner/internal/database" // New import
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner" // New import
	"ai-meal-planner/internal/recipe"  // New import
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
	analystModel := llm.NewGroqClient(cfg, llm.ModelAnalyst, 0.1)
	normalizerModel := llm.NewGroqClient(cfg, llm.ModelNormalizer, 0.3)

	geminiClient, err := llm.NewGeminiClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

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

	// 3. Initialize Ghost Client
	ghostClient := ghost.NewClient(cfg)

	metricsStore := metrics.NewStore(db.SQL)
	defer metricsStore.Close()

	// 5. Initialize Services
	mealPlanner := planner.NewPlanner(recipeRepo, vectorRepo, planRepo, analystModel, normalizerModel, geminiClient)
	recipeClipper := clipper.NewClipper(ghostClient, normalizerModel)

	// 6. Initialize Telegram Bot
	bot, err := telegram.NewBot(cfg, mealPlanner, recipeClipper, metricsStore, normalizerModel, geminiClient, planRepo, recipeRepo, vectorRepo)
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
