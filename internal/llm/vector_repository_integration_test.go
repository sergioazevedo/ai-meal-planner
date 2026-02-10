package llm_test

import (
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/vector_db"
	"ai-meal-planner/internal/recipe" // Import the recipe package
	"context"
	"database/sql"
	"log"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// goldenTest defines a test case for vector search evaluation.
type goldenTest struct {
	name        string
	query       string
	expectedIDs []string
	minRecall   float64 // Minimum acceptable recall score (0.0 to 1.0)
}

// TestVectorSearchRecallIntegration performs an integration test for vector search
// with a cached embedding generator to reduce API calls.
func TestVectorSearchRecallIntegration(t *testing.T) {
	// --- Test Setup ---
	ctx := context.Background()

	// 1. Load configuration
	cfg, err := config.NewFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config: %v\n", err)
	}

	// Determine which LLM client to use based on config
	var realEmbeddingGenerator llm.EmbeddingGenerator
	var realTextGenerator llm.TextGenerator // Also need TextGenerator for NormalizeHTML
	if cfg.GeminiAPIKey != "" {
		geminiClient, err := llm.NewGeminiClient(ctx, cfg)
		if err != nil {
			t.Skipf("Skipping Gemini integration test: failed to create Gemini client: %v\n", err)
		}
		defer geminiClient.Close()
		realEmbeddingGenerator = geminiClient
		realTextGenerator = geminiClient // Gemini client implements both
		t.Log("Using Gemini for embedding and text generation.\n")
	} else if cfg.GroqAPIKey != "" {
		groqClient := llm.NewGroqClient(cfg, llm.ModelNormalizer)
		realTextGenerator = groqClient
		// For Groq, if it doesn't have an embedding model, we might need a dummy or skip
		// For this test, let's assume embedding is always realEmbeddingGenerator for now
		t.Log("Using Groq for text generation (no embedding model).\n")
		// If Groq only for text, need a fallback for embedding.
		t.Skip("Skipping integration test: Groq does not provide embedding models suitable for this test setup. Please configure GEMINI_API_KEY.\n")
	} else {
		t.Skip("Skipping integration test: No GEMINI_API_KEY or GROQ_API_KEY found in environment.\n")
	}

	// Set up cache for embeddings
	cacheDir := t.TempDir() // Use a temporary directory for the cache
	cacheFilePath := filepath.Join(cacheDir, "embeddings_cache.json")
	cachedEmbGen, err := llm.NewCachedEmbeddingGenerator(realEmbeddingGenerator, cacheFilePath)
	if err != nil {
		t.Fatalf("Failed to create cached embedding generator: %v\n", err)
	}
	t.Cleanup(func() {
		if err := cachedEmbGen.SaveCache(); err != nil {
			log.Printf("Failed to save embedding cache: %v\n", err)
		}
	})

	// Setup in-memory SQLite database
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v\n", err)
	}
	defer db.Close()

	// Create tables for embeddings
	_, err = db.Exec(vectordb.Schema) // Corrected to vectordb.Schema
	if err != nil {
		t.Fatalf("Failed to execute schema: %v\n", err)
	}

	vectorRepo := llm.NewVectorRepository(db)
	extractor := recipe.NewExtractor(realTextGenerator, cachedEmbGen, vectorRepo) // New Extractor instance

	// --- Golden Set Definition ---
	// IMPORTANT: You MUST replace this with your actual queries and expected RecipeIDs.
	// Also, ensure the dummy recipes below reflect these expected IDs.
	goldenTests := []goldenTest{
		{
			name:        "Spicy Pasta Dish",
			query:       "quick pasta with spicy sauce",
			expectedIDs: []string{"recipe-pasta-arrabbiata"},
			minRecall:   1.0, // Expect to find all expected recipes
		},
		{
			name:        "Healthy Chicken Salad",
			query:       "light chicken salad for lunch",
			expectedIDs: []string{"recipe-chicken-salad-mediterranean"},
			minRecall:   1.0,
		},
		{
			name:        "Vegetarian Chili",
			query:       "hearty vegetarian meal with beans",
			expectedIDs: []string{"recipe-vegetarian-chili"},
			minRecall:   1.0,
		},
	}

	// --- Populate Test Database with Dummy Recipes & Embeddings ---
	// These are dummy recipes that should align with your goldenTests.
	// The ToEmbeddingText() method from internal/recipe/recipe.go is used.
	dummyRecipes := []recipe.Recipe{
		{
			ID:          "recipe-pasta-arrabbiata",
			Title:       "Spicy Penne Arrabbiata",
			Ingredients: []string{"penne", "tomatoes", "garlic", "chilli flakes", "parsley"},
			Tags:        []string{"italian", "pasta", "spicy", "quick", "vegetarian"},
			PrepTime:    "15 min",
		},
		{
			ID:          "recipe-chicken-salad-mediterranean",
			Title:       "Mediterranean Chicken Salad",
			Ingredients: []string{"chicken breast", "cucumber", "tomato", "olives", "feta cheese", "lemon dressing"},
			Tags:        []string{"healthy", "salad", "chicken", "mediterranean", "lunch"},
			PrepTime:    "20 min",
		},
		{
			ID:          "recipe-vegetarian-chili",
			Title:       "Hearty Three-Bean Vegetarian Chili",
			Ingredients: []string{"kidney beans", "black beans", "pinto beans", "tomatoes", "peppers", "onions", "chili powder"},
			Tags:        []string{"vegetarian", "comfort food", "chili", "easy"},
			PrepTime:    "45 min, cook time 25 min"},
		// Add more dummy recipes here to test various scenarios
	}

	// Generate and save embeddings for dummy recipes using NormalizeHTML
	for _, r := range dummyRecipes {
		// NormalizeHTML now handles only extraction
		extractorResult, err := extractor.ExtractRecipe( // Use extractor instance
			ctx,
			recipe.PostData{
				ID:        r.ID,
				Title:     r.Title,
				UpdatedAt: r.UpdatedAt,
				HTML:      "<html><body>Dummy HTML</body></html>", // Dummy HTML content
			},
		)
		if err != nil {
			t.Fatalf("Failed to extract recipe %s: %v\n", r.ID, err)
		}

		// Process and save embedding separately
		_, _, err = extractor.ProcessAndSaveEmbedding(ctx, extractorResult.Recipe) // Use extractor instance
		if err != nil {
			t.Fatalf("Failed to process and save embedding for recipe %s: %v\n", r.ID, err)
		}
	}

	// --- Run Evaluation ---
	for _, gt := range goldenTests {
		t.Run(gt.name, func(t *testing.T) {
			queryEmbedding, err := cachedEmbGen.GenerateEmbedding(ctx, gt.query)
			if err != nil {
				t.Fatalf("Failed to generate embedding for query \"%s\": %v\n", gt.query, err)
			}

			// Find similar recipes. We'll retrieve a larger number than expected
			// to calculate recall@K, where K is the number of expected IDs.
			retrievedIDs, err := vectorRepo.FindSimilar(ctx, queryEmbedding, 10, nil) // Retrieve top 10
			if err != nil {
				t.Fatalf("Failed to find similar recipes for query \"%s\": %v\n", gt.query, err)
			}

			// Calculate Recall@K
			foundCount := 0
			for _, expectedID := range gt.expectedIDs {
				if slices.Contains(retrievedIDs, expectedID) {
					foundCount++
				}
			}

			recall := float64(foundCount) / float64(len(gt.expectedIDs))
			t.Logf("Query: \"%s\", Expected: %v, Retrieved (Top %d): %v, Recall: %.2f\n",
				gt.query, gt.expectedIDs, len(retrievedIDs), retrievedIDs, recall)

			if recall < gt.minRecall {
				t.Errorf("Recall (%.2f) for query \"%s\" is below minimum acceptable (%.2f)\n", recall, gt.query, gt.minRecall)
			}
		})
	}
}

// Ensure vector_db.Schema is available. If it's embedded, it needs to be accessible.
// If vector_db.Schema is not directly exposed, we might need to copy its content here or
// adjust internal/llm/vector_db/db.go to export it.
// Assuming for now it's either exported or we can read it.
// From the folder structure, vector_db has a db.go, embedding_queries.sql.go, and models.go.
// The schema is likely in db.go or embedding_queries.sql.
// Let's assume vector_db.Schema is available. If not, I'll adapt.

// Note: To run this test, set either GEMINI_API_KEY or GROQ_API_KEY in your environment.
// Example: export GEMINI_API_KEY="your_key" && go test -v ./internal/llm/...
