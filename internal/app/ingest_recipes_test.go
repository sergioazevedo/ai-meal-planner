package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"

	_ "github.com/mattn/go-sqlite3"
)

type mockGhostClientForIngest struct {
	posts []ghost.Post
	err   error
}

func (m *mockGhostClientForIngest) FetchRecipes() ([]ghost.Post, error) {
	return m.posts, m.err
}

func (m *mockGhostClientForIngest) CreatePost(title, html string, tags []string, publish bool) (*ghost.Post, error) {
	return nil, nil
}

func TestIngestRecipes_Cleanup(t *testing.T) {
	ctx := context.Background()

	// 1. Setup DB
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create tables with Foreign Keys enabled
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE recipes (id TEXT PRIMARY KEY, data TEXT, updated_at DATETIME);
		CREATE TABLE recipe_embeddings (recipe_id TEXT PRIMARY KEY, embedding BLOB, text_hash TEXT, FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE);
		CREATE TABLE execution_metrics (id INTEGER PRIMARY KEY, agent_name TEXT, model TEXT, prompt_tokens INTEGER, completion_tokens INTEGER, latency_ms INTEGER, timestamp DATETIME);
	`)
	if err != nil {
		t.Fatal(err)
	}

	recipeRepo := recipe.NewRepository(db)
	vectorRepo := llm.NewVectorRepository(db)
	metricsStore := metrics.NewStore(db)

	// 2. Pre-populate DB with an orphaned recipe
	orphanedID := "orphaned-1"
	err = recipeRepo.Save(ctx, recipe.Recipe{
		ID:    orphanedID,
		Title: "Orphaned Recipe",
	})
	if err != nil {
		t.Fatalf("failed to save orphaned recipe: %v", err)
	}

	// Also add an embedding for it to check CASCADE
	err = vectorRepo.Save(ctx, orphanedID, []float32{1.0}, "hash")
	if err != nil {
		t.Fatalf("failed to save orphaned embedding: %v", err)
	}

	// 3. Setup App with Mock Ghost Client
	stayingID := "staying-1"
	mockGhost := &mockGhostClientForIngest{
		posts: []ghost.Post{
			{
				ID:        stayingID,
				Title:     "Staying Recipe",
				UpdatedAt: time.Now().Format(time.RFC3339),
				HTML:      "<html></html>",
			},
		},
	}

	recipeJSON := `{"id": "staying-1", "title": "Staying Recipe", "ingredients": ["A"], "instructions": ["B"]}`
	textGen := &mockTextGen{res: recipeJSON}
	embGen := &mockEmbGen{}

	app := &App{
		ghostClient:  mockGhost,
		recipeRepo:   recipeRepo,
		vectorRepo:   vectorRepo,
		metricsStore: metricsStore,
		extractor:    recipe.NewExtractor(textGen, embGen, vectorRepo),
	}

	// 4. Run IngestRecipes
	// Note: This will sleep for 5 seconds because of the hardcoded sleep in IngestRecipes
	err = app.IngestRecipes(ctx, false)
	if err != nil {
		t.Fatalf("IngestRecipes failed: %v", err)
	}

	// 5. Verify results
	// Staying recipe should exist
	_, err = recipeRepo.Get(ctx, stayingID)
	if err != nil {
		t.Errorf("Expected staying recipe to exist, got error: %v", err)
	}

	// Orphaned recipe should be gone
	_, err = recipeRepo.Get(ctx, orphanedID)
	if err == nil {
		t.Errorf("Expected orphaned recipe to be deleted, but it still exists")
	}

	// Orphaned embedding should be gone (via CASCADE)
	emb, err := vectorRepo.Get(ctx, orphanedID)
	if err != nil {
		t.Errorf("vectorRepo.Get failed: %v", err)
	}
	if emb != nil {
		t.Errorf("Expected orphaned embedding to be deleted, but it still exists")
	}
}
