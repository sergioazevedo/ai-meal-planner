package llm_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/value"

	_ "modernc.org/sqlite"
)

const (
	retrievalTopK    = 3
	minimumHitAt1    = 0.60
	minimumRecallAt3 = 0.70
	minimumMRRAt3    = 0.70
	freeTierInterval = 1100 * time.Millisecond
	retrievalTimeout = 3 * time.Minute
)

func TestGoldenRetrievalDataset(t *testing.T) {
	recipes := loadFixture[[]value.Recipe](t, "rag_eval_recipes.json")
	queries := loadFixture[[]goldenQuery](t, "retrieval_queries.json")
	validateGoldenDataset(t, recipes, queries)
}

// The fixtures are deliberately curated and manually labelled. Keep ambiguous
// alternatives in the recipe set, and never derive relevance labels from the
// retrieval system being evaluated.
// TestVectorSearchQualityIntegration evaluates real embeddings against a stable,
// curated recipe dataset. It is intentionally a live, opt-in test.
func TestVectorSearchQualityIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live retrieval evaluation in short mode")
	}

	embeddingAPIKey := os.Getenv("EMBEDDING_API_KEY")
	if embeddingAPIKey == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("EMBEDDING_API_KEY must be configured for the retrieval eval in CI")
		}
		t.Skip("skipping live retrieval evaluation: EMBEDDING_API_KEY is not set")
	}

	recipes := loadFixture[[]value.Recipe](t, "rag_eval_recipes.json")
	queries := loadFixture[[]goldenQuery](t, "retrieval_queries.json")
	validateGoldenDataset(t, recipes, queries)

	dbPath := filepath.Join(t.TempDir(), "retrieval-eval.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	defer db.Close()
	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	embeddingClient := llm.NewEmbeddingClient(&config.Config{EmbeddingAPIKey: embeddingAPIKey})
	defer embeddingClient.Close()
	recipeRepo := recipe.NewRepository(db.SQL)
	vectorRepo := llm.NewVectorRepository(db.SQL)
	extractor := recipe.NewExtractor(nil, embeddingClient, vectorRepo)
	ctx, cancel := context.WithTimeout(context.Background(), retrievalTimeout)
	defer cancel()

	for _, rec := range recipes {
		if err := recipeRepo.Save(ctx, rec); err != nil {
			t.Fatalf("save recipe %q: %v", rec.Title, err)
		}
		if _, _, err := extractor.ProcessAndSaveEmbedding(ctx, rec, false); err != nil {
			t.Fatalf("embed recipe %q: %v", rec.Title, err)
		}
		// The live evaluation makes 64 embedding requests. Leave a gap between
		// them so the curated suite remains usable with the provider's free tier.
		if err := waitForEmbeddingSlot(ctx, freeTierInterval); err != nil {
			t.Fatalf("pace embedding requests: %v", err)
		}
	}

	titlesByID := make(map[string]string, len(recipes))
	for _, rec := range recipes {
		titlesByID[rec.ID] = rec.Title
	}

	results := make([]rankedResult, 0, len(queries))
	for i, query := range queries {
		queryEmbedding, err := embeddingClient.GenerateEmbedding(ctx, query.Query)
		if err != nil {
			t.Fatalf("embed query %q: %v", query.Name, err)
		}
		retrievedIDs, err := vectorRepo.FindSimilar(ctx, queryEmbedding, retrievalTopK, nil)
		if err != nil {
			t.Fatalf("retrieve query %q: %v", query.Name, err)
		}

		results = append(results, rankedResult{Query: query, RetrievedIDs: retrievedIDs})
		t.Logf("%s: %v", query.Name, rankedTitles(retrievedIDs, titlesByID))
		if i < len(queries)-1 {
			if err := waitForEmbeddingSlot(ctx, freeTierInterval); err != nil {
				t.Fatalf("pace embedding requests: %v", err)
			}
		}
	}

	metrics := calculateRetrievalMetrics(results, retrievalTopK)
	t.Logf(
		"retrieval quality across %d queries: Hit@1=%.3f Recall@3=%.3f MRR@3=%.3f",
		len(queries),
		metrics.HitAt1,
		metrics.RecallAtK,
		metrics.MRRAtK,
	)

	if metrics.HitAt1 < minimumHitAt1 {
		t.Errorf("Hit@1 %.3f is below minimum %.2f", metrics.HitAt1, minimumHitAt1)
	}
	if metrics.RecallAtK < minimumRecallAt3 {
		t.Errorf("Recall@3 %.3f is below minimum %.2f", metrics.RecallAtK, minimumRecallAt3)
	}
	if metrics.MRRAtK < minimumMRRAt3 {
		t.Errorf("MRR@3 %.3f is below minimum %.2f", metrics.MRRAtK, minimumMRRAt3)
	}
}

func waitForEmbeddingSlot(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func loadFixture[T any](t *testing.T, name string) T {
	t.Helper()

	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}

	var fixture T
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", path, err)
	}
	return fixture
}

func validateGoldenDataset(t *testing.T, recipes []value.Recipe, queries []goldenQuery) {
	t.Helper()

	if len(recipes) < 30 {
		t.Fatalf("golden dataset has %d recipes; want at least 30", len(recipes))
	}
	if len(queries) < 10 {
		t.Fatalf("golden dataset has %d queries; want at least 10", len(queries))
	}

	recipeIDs := make(map[string]struct{}, len(recipes))
	for _, rec := range recipes {
		if rec.ID == "" || rec.Title == "" || len(rec.Ingredients) == 0 {
			t.Fatalf("incomplete recipe fixture: ID=%q title=%q", rec.ID, rec.Title)
		}
		if _, exists := recipeIDs[rec.ID]; exists {
			t.Fatalf("duplicate recipe ID %q", rec.ID)
		}
		recipeIDs[rec.ID] = struct{}{}
	}

	for _, query := range queries {
		if query.Name == "" || query.Query == "" || len(query.RelevantIDs) == 0 {
			t.Fatalf("incomplete golden query: %+v", query)
		}
		for _, id := range query.RelevantIDs {
			if _, exists := recipeIDs[id]; !exists {
				t.Fatalf("query %q references unknown recipe ID %q", query.Name, id)
			}
		}
	}
}

func rankedTitles(ids []string, titlesByID map[string]string) []string {
	titles := make([]string, 0, len(ids))
	for _, id := range ids {
		title := titlesByID[id]
		if title == "" {
			title = id
		}
		titles = append(titles, title)
	}
	return titles
}
