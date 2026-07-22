package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/llmtest"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/value"

	_ "modernc.org/sqlite"
)

type mockGhostClientForIngest struct {
	posts      []ghost.Post
	recipeByID *ghost.Post
	err        error
}

func (m *mockGhostClientForIngest) FetchRecipes() ([]ghost.Post, error) {
	return m.posts, m.err
}

func (m *mockGhostClientForIngest) FetchRecipeByID(id string) (*ghost.Post, error) {
	return m.recipeByID, m.err
}

func TestRetagRecipeByIDOnlyRegeneratesTagsAndEmbedding(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "retag.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB() error = %v", err)
	}
	defer db.Close()
	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("MigrateUp() error = %v", err)
	}

	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	metricsStore := metrics.NewStore(db.SQL)
	original := value.Recipe{
		ID:          "salmon-1",
		Title:       "Salmão com brócolis",
		Ingredients: []string{"200 g salmão", "5 ramos de brócolis"},
		Tags:        []string{"salmão", "fish", "vegetariano"},
		UpdatedAt:   "2026-07-22T18:00:00Z",
	}
	if err := recipeRepo.Save(ctx, original); err != nil {
		t.Fatalf("save original recipe: %v", err)
	}

	embGen := &llmtest.MockEmbeddingGenerator{Values: []float32{0.1, 0.2}}
	application := &App{
		ghostClient: &mockGhostClientForIngest{recipeByID: &ghost.Post{
			ID:   original.ID,
			Tags: []ghost.Tag{{Name: "Air Fryer"}},
		}},
		recipeRepo:   recipeRepo,
		metricsStore: metricsStore,
		extractor:    recipe.NewExtractor(nil, embGen, vectorRepo),
		tagger: recipe.NewTagger(&llmtest.MockTextGenerator{Response: `{
			"tags":[
				{"pt":"salmão","en":"salmon"},
				{"pt":"brócolis","en":"broccoli"},
				{"pt":"fritadeira sem óleo","en":"air fryer"}
			]
		}`}),
	}

	if err := application.RetagRecipeByID(ctx, original.ID); err != nil {
		t.Fatalf("RetagRecipeByID() error = %v", err)
	}

	got, err := recipeRepo.Get(ctx, original.ID)
	if err != nil {
		t.Fatalf("get retagged recipe: %v", err)
	}
	wantTags := []string{"salmão", "salmon", "brócolis", "broccoli", "fritadeira sem óleo", "air fryer"}
	if !slices.Equal(got.Tags, wantTags) {
		t.Fatalf("tags = %#v, want %#v", got.Tags, wantTags)
	}
	if got.Title != original.Title || !slices.Equal(got.Ingredients, original.Ingredients) {
		t.Fatalf("retag changed normalized recipe fields: %#v", got)
	}
	if embGen.Calls != 1 {
		t.Fatalf("embedding calls = %d, want 1", embGen.Calls)
	}
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
	err = recipeRepo.Save(ctx, value.Recipe{
		ID:    orphanedID,
		Title: "Orphaned Recipe",
	})
	if err != nil {
		t.Fatalf("failed to save orphaned recipe: %v", err)
	}

	// Also add an embedding for it to check CASCADE
	err = vectorRepo.Save(ctx, orphanedID, []float32{1.0}, "hash", llm.EmbeddingMetadata{
		Model:      "test-embedding-model",
		Dimensions: 1,
	})
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

	recipeJSON := `{"id": "staying-1", "title": "Staying Recipe", "ingredients": ["A"]}`
	textGen := &llmtest.MockTextGenerator{Response: recipeJSON}
	embGen := &llmtest.MockEmbeddingGenerator{}

	app := &App{
		ghostClient:  mockGhost,
		recipeRepo:   recipeRepo,
		vectorRepo:   vectorRepo,
		metricsStore: metricsStore,
		extractor:    recipe.NewExtractor(textGen, embGen, vectorRepo),
		tagger:       recipe.NewTagger(&llmtest.MockTextGenerator{Response: `{"tags":[{"pt":"receita","en":"recipe"}]}`}),
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
