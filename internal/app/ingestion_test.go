package app

import (
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

type mockTextGen struct {
	res string
}

func (m *mockTextGen) GenerateContent(ctx context.Context, prompt string) (llm.ContentResponse, error) {
	return llm.ContentResponse{Content: m.res}, nil
}

type mockEmbGen struct{}

func (m *mockEmbGen) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}

func TestProcessAndSaveRecipe(t *testing.T) {
	ctx := context.Background()

	// Setup In-Memory DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE recipes (id TEXT PRIMARY KEY, data TEXT, updated_at DATETIME);
		CREATE TABLE recipe_embeddings (recipe_id TEXT PRIMARY KEY, embedding BLOB, text_hash TEXT);
		CREATE TABLE execution_metrics (id INTEGER PRIMARY KEY, agent_name TEXT, model TEXT, prompt_tokens INTEGER, completion_tokens INTEGER, latency_ms INTEGER, timestamp DATETIME);
	`)
	if err != nil {
		t.Fatal(err)
	}

	recipeRepo := recipe.NewRepository(db)
	vectorRepo := llm.NewVectorRepository(db)
	metricsStore := metrics.NewStore(db)

	recipeJSON := `{"title": "Test Recipe", "ingredients": ["A"], "instructions": ["B"]}`
	textGen := &mockTextGen{res: recipeJSON}
	embGen := &mockEmbGen{}
	extractor := recipe.NewExtractor(textGen, embGen, vectorRepo)

	post := ghost.Post{
		ID:        "p1",
		Title:     "Post 1",
		UpdatedAt: "2023-01-01T00:00:00Z",
		HTML:      "<html></html>",
	}

	t.Run("New Recipe", func(t *testing.T) {
		err := ProcessAndSaveRecipe(ctx, extractor, recipeRepo, metricsStore, post)
		if err != nil {
			t.Fatalf("ProcessAndSaveRecipe failed: %v", err)
		}

		// Verify recipe saved
		rec, err := recipeRepo.Get(ctx, "p1")
		if err != nil {
			t.Fatalf("Failed to get recipe: %v", err)
		}
		if rec.Title != "Test Recipe" {
			t.Errorf("Expected title 'Test Recipe', got '%s'", rec.Title)
		}

		// Verify embedding saved
		emb, err := vectorRepo.Get(ctx, "p1")
		if err != nil {
			t.Fatalf("Failed to get embedding: %v", err)
		}
		if emb == nil || len(emb.Embedding) != 2 {
			t.Errorf("Expected embedding length 2, got %v", emb)
		}
	})
}
