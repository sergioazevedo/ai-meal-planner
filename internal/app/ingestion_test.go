package app

import (
	"context"
	"os"
	"testing"

	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/llmtest"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"

	_ "modernc.org/sqlite"
)

func TestProcessAndSaveRecipe(t *testing.T) {
	ctx := context.Background()

	// Setup temporary DB
	tempFile, err := os.CreateTemp("", "ingestion_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbPath := tempFile.Name()
	tempFile.Close()
	defer os.Remove(dbPath)

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	metricsStore := metrics.NewStore(db.SQL)

	recipeJSON := `{"title": "Test Recipe", "ingredients": ["A"]}`
	textGen := &llmtest.MockTextGenerator{Response: recipeJSON}
	embGen := &llmtest.MockEmbeddingGenerator{Values: []float32{0.1, 0.2}}
	extractor := recipe.NewExtractor(textGen, embGen, vectorRepo)

	post := ghost.Post{
		ID:        "1",
		Title:     "Test Recipe",
		UpdatedAt: "2023-01-01T00:00:00Z",
		HTML:      "<html></html>",
	}

	t.Run("New Recipe", func(t *testing.T) {
		err := ProcessAndSaveRecipe(ctx, extractor, recipeRepo, metricsStore, post, false)
		if err != nil {
			t.Fatalf("ProcessAndSaveRecipe failed: %v", err)
		}

		// Verify recipe saved
		rec, err := recipeRepo.Get(ctx, "1")
		if err != nil {
			t.Errorf("Failed to get recipe: %v", err)
		}
		if rec.Title != "Test Recipe" {
			t.Errorf("Expected title 'Test Recipe', got '%s'", rec.Title)
		}

		// Verify embedding saved
		embRecord, err := vectorRepo.Get(ctx, "1")
		if err != nil {
			t.Errorf("Failed to get embedding: %v", err)
		}
		if embRecord == nil || len(embRecord.Embedding) != 2 {
			count := 0
			if embRecord != nil {
				count = len(embRecord.Embedding)
			}
			t.Errorf("Expected embedding length 2, got %d", count)
		}
	})
}
