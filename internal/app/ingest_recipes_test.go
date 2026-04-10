package app

import (
	"context"
	"os"
	"testing"
	"time"

	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/llmtest"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"

	_ "modernc.org/sqlite"
)

type mockGhostClientForIngest struct {
	posts []ghost.Post
	err   error
}

func (m *mockGhostClientForIngest) FetchRecipes() ([]ghost.Post, error) {
	return m.posts, m.err
}

func (m *mockGhostClientForIngest) FetchRecipeByID(id string) (*ghost.Post, error) {
	return nil, nil
}

func (m *mockGhostClientForIngest) CreatePost(title, html string, tags []string, publish bool) (*ghost.Post, error) {
	return nil, nil
}

func TestIngestRecipes_Cleanup(t *testing.T) {
	ctx := context.Background()

	// 1. Setup DB
	tempFile, err := os.CreateTemp("", "ingest_test_*.db")
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

	// Enable Foreign Keys for CASCADE DELETE to work
	if _, err := db.SQL.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	metricsStore := metrics.NewStore(db.SQL)

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
	textGen := &llmtest.MockTextGenerator{Response: recipeJSON}
	embGen := &llmtest.MockEmbeddingGenerator{}

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
