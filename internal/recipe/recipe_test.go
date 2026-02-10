package recipe

import (
	"ai-meal-planner/internal/llm"
	"context"
	"crypto/md5"   // New import
	"database/sql" // Added for sql.ErrNoRows
	"encoding/hex" // New import
	"errors"
	"strings"
	"testing"
)

type MockEmbedingGenerator struct {
	shouldError bool
}

type MockTextGenerator struct {
	shouldError bool
	response    string
}

type MockVectorRepository struct {
	// For testing specific scenarios if needed
	mockGet     func(ctx context.Context, recipeID string) (*llm.EmbeddingRecord, error)
	mockSave    func(ctx context.Context, recipeID string, embedding []float32, textHash string) error
	shouldError bool
}

// Implement llm.VectorRepositoryInterface for MockVectorRepository
func (m *MockVectorRepository) Get(ctx context.Context, recipeID string) (*llm.EmbeddingRecord, error) {
	if m.shouldError {
		return nil, errors.New("mock vector repo error")
	}
	if m.mockGet != nil {
		return m.mockGet(ctx, recipeID)
	}
	return nil, sql.ErrNoRows // Default: no existing record
}

func (m *MockVectorRepository) Save(ctx context.Context, recipeID string, embedding []float32, textHash string) error {
	if m.shouldError {
		return errors.New("mock vector repo error")
	}
	if m.mockSave != nil {
		return m.mockSave(ctx, recipeID, embedding, textHash)
	}
	return nil // Default: save successfully
}

func (m *MockVectorRepository) FindSimilar(ctx context.Context, queryEmbedding []float32, limit int, excludeIDs []string) ([]string, error) {
	return nil, nil // Not relevant for NormalizeHTML tests
}

func (m *MockVectorRepository) WithTx(tx *sql.Tx) *llm.VectorRepository {
	return nil // Return nil for mock as real transaction not needed for this mock
}

func (m *MockTextGenerator) GenerateContent(_ context.Context, _ string) (llm.ContentResponse, error) {
	if m.shouldError {
		return llm.ContentResponse{}, errors.New("LLM error")
	}
	return llm.ContentResponse{Content: m.response}, nil
}

func (m *MockEmbedingGenerator) GenerateEmbedding(_ context.Context, _ string) ([]float32, error) {
	if m.shouldError {
		return nil, errors.New("LLM error")
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func TestExtractor_ExtractRecipe(t *testing.T) {
	ctx := context.Background()
	post := PostData{
		ID:    "1",
		Title: "Test Recipe",
		HTML:  "<h1>Test Recipe</h1><p>Ingredients: ...</p>",
	}

	t.Run("Success", func(t *testing.T) {
		mockTextGeneration := &MockTextGenerator{
			response: `{
				"title": "Test Recipe",
				"ingredients": ["Ingredient 1", "Ingredient 2"],
				"instructions": ["Step 1. Do something."],
				"tags": ["test", "recipe"],
				"prep_time": "30 mins",
				"servings": "4"
			}`,
		}
		// Extractor doesn't directly use EmbeddingGenerator or VectorRepository for ExtractRecipe
		extractor := NewExtractor(mockTextGeneration, nil, nil)

		extractorResult, err := extractor.ExtractRecipe(ctx, post)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if extractorResult.Recipe.ID != "1" {
			t.Errorf("Expected ID '1', got '%s'", extractorResult.Recipe.ID)
		}
		if extractorResult.Recipe.Title != "Test Recipe" {
			t.Errorf("Expected title 'Test Recipe', got '%s'", extractorResult.Recipe.Title)
		}
		if len(extractorResult.Recipe.Ingredients) != 2 {
			t.Errorf("Expected 2 ingredients, got %d", len(extractorResult.Recipe.Ingredients))
		}
		if len(extractorResult.Recipe.Instructions) != 1 || extractorResult.Recipe.Instructions[0] != "Step 1. Do something." {
			t.Errorf("Expected instructions ['Step 1. Do something.'], got %v", extractorResult.Recipe.Instructions)
		}
		if len(extractorResult.Recipe.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(extractorResult.Recipe.Tags))
		}
		if extractorResult.Recipe.PrepTime != "30 mins" {
			t.Errorf("Expected PrepTime '30 mins', got '%s'", extractorResult.Recipe.PrepTime)
		}
		if extractorResult.Recipe.Servings != "4" {
			t.Errorf("Expected Servings '4', got '%s'", extractorResult.Recipe.Servings)
		}
		if extractorResult.Meta.AgentName != "Extractor" {
			t.Errorf("Expected agent name 'Extractor', got '%s'", extractorResult.Meta.AgentName)
		}
	})

	t.Run("LLMError", func(t *testing.T) {
		mockTextGeneration := &MockTextGenerator{shouldError: true}
		extractor := NewExtractor(mockTextGeneration, nil, nil)

		_, err := extractor.ExtractRecipe(ctx, post)
		if err == nil {
			t.Fatal("Expected an error from the LLM client, got nil")
		}
		expectedError := "failed to get LLM response: LLM error"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		mockTextGeneration := &MockTextGenerator{response: "this is not json"}
		extractor := NewExtractor(mockTextGeneration, nil, nil)

		_, err := extractor.ExtractRecipe(ctx, post)
		if err == nil {
			t.Fatal("Expected an error for invalid JSON, got nil")
		}
		if !strings.HasPrefix(err.Error(), "failed to get LLM response: failed to unmarshal LLM response") {
			t.Errorf("Expected a JSON unmarshaling error, got: %v", err)
		}
	})
}

func TestExtractor_ProcessAndSaveEmbedding(t *testing.T) {
	ctx := context.Background()
	sampleRecipe := Recipe{
		ID:          "test-recipe-id",
		Title:       "Sample Recipe",
		Ingredients: []string{"ing1", "ing2"},
		Tags:        []string{"tag1"},
	}

	t.Run("Success_CacheMiss", func(t *testing.T) {
		mockEmbGen := &MockEmbedingGenerator{}
		mockVectorRepo := &MockVectorRepository{
			mockGet: func(ctx context.Context, recipeID string) (*llm.EmbeddingRecord, error) {
				return nil, sql.ErrNoRows // Simulate cache miss
			},
		}
		extractor := NewExtractor(nil, mockEmbGen, mockVectorRepo)

		embedding, meta, err := extractor.ProcessAndSaveEmbedding(ctx, sampleRecipe)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(embedding) == 0 {
			t.Error("Expected embedding to be generated, got empty")
		}
		if meta.AgentName != "Embedding" {
			t.Errorf("Expected agent name 'Embedding', got '%s'", meta.AgentName)
		}
		if meta.Usage.PromptTokens == 0 {
			t.Error("Expected prompt tokens to be > 0 for cache miss")
		}
	})

	t.Run("Success_CacheHit", func(t *testing.T) {
		sampleText := sampleRecipe.ToEmbeddingText()
		hasher := md5.New()
		hasher.Write([]byte(sampleText))
		currentTextHash := hex.EncodeToString(hasher.Sum(nil))

		mockEmbGen := &MockEmbedingGenerator{} // Should not be called
		mockVectorRepo := &MockVectorRepository{
			mockGet: func(ctx context.Context, recipeID string) (*llm.EmbeddingRecord, error) {
				return &llm.EmbeddingRecord{
					Embedding: []float32{0.1, 0.2, 0.3},
					TextHash:  currentTextHash, // Simulate cache hit
				}, nil
			},
		}
		extractor := NewExtractor(nil, mockEmbGen, mockVectorRepo)

		embedding, meta, err := extractor.ProcessAndSaveEmbedding(ctx, sampleRecipe)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(embedding) == 0 {
			t.Error("Expected embedding to be returned, got empty")
		}
		if meta.AgentName != "Embedding" {
			t.Errorf("Expected agent name 'Embedding', got '%s'", meta.AgentName)
		}
		if meta.Usage.PromptTokens != 0 {
			t.Error("Expected prompt tokens to be 0 for cache hit")
		}
	})

	t.Run("EmbeddingGenerationError", func(t *testing.T) {
		mockEmbGen := &MockEmbedingGenerator{shouldError: true}
		mockVectorRepo := &MockVectorRepository{
			mockGet: func(ctx context.Context, recipeID string) (*llm.EmbeddingRecord, error) {
				return nil, sql.ErrNoRows // Simulate cache miss
			},
		}
		extractor := NewExtractor(nil, mockEmbGen, mockVectorRepo)

		_, _, err := extractor.ProcessAndSaveEmbedding(ctx, sampleRecipe)
		if err == nil {
			t.Fatal("Expected an error during embedding generation, got nil")
		}
		expectedError := "failed to generate embedding: LLM error"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("VectorRepoSaveError", func(t *testing.T) {
		mockEmbGen := &MockEmbedingGenerator{}
		mockVectorRepo := &MockVectorRepository{shouldError: true} // Simulate save error
		extractor := NewExtractor(nil, mockEmbGen, mockVectorRepo)

		_, _, err := extractor.ProcessAndSaveEmbedding(ctx, sampleRecipe)
		if err == nil {
			t.Fatal("Expected an error during vector repository save, got nil")
		}
		expectedError := "failed to save embedding with hash: mock vector repo error"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})
}
