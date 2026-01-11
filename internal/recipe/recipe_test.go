package recipe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"ai-meal-planner/internal/ghost"
)

// mockLLMClient is a mock implementation of the llm.LLMClient interface for testing.
type mockLLMClient struct {
	response    string
	shouldError bool
}

func (m *mockLLMClient) GenerateContent(ctx context.Context, prompt string) (string, error) {
	if m.shouldError {
		return "", errors.New("LLM error")
	}
	return m.response, nil
}

func (m *mockLLMClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.shouldError {
		return nil, errors.New("LLM error")
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockLLMClient) Close() error {
	return nil
}

func TestNormalizeRecipeHTML(t *testing.T) {
	ctx := context.Background()
	post := ghost.Post{
		ID:    "1",
		Title: "Test Recipe",
		HTML:  "<h1>Test Recipe</h1><p>Ingredients: ...</p>",
	}

	t.Run("Success", func(t *testing.T) {
		mockClient := &mockLLMClient{
			response: `{
				"title": "Test Recipe",
				"ingredients": ["Ingredient 1", "Ingredient 2"],
				"instructions": "Step 1. Do something.",
				"tags": ["test", "recipe"],
				"prep_time": "30 mins",
				"servings": "4"
			}`,
		}

		normalizedRecipe, err := NormalizeRecipeHTML(ctx, mockClient, post)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if normalizedRecipe.Title != "Test Recipe" {
			t.Errorf("Expected title 'Test Recipe', got '%s'", normalizedRecipe.Title)
		}
		if len(normalizedRecipe.Ingredients) != 2 {
			t.Errorf("Expected 2 ingredients, got %d", len(normalizedRecipe.Ingredients))
		}
		if normalizedRecipe.Instructions != "Step 1. Do something." {
			t.Errorf("Expected instructions 'Step 1. Do something.', got '%s'", normalizedRecipe.Instructions)
		}
		if len(normalizedRecipe.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(normalizedRecipe.Tags))
		}
		if normalizedRecipe.PrepTime != "30 mins" {
			t.Errorf("Expected PrepTime '30 mins', got '%s'", normalizedRecipe.PrepTime)
		}
		if normalizedRecipe.Servings != "4" {
			t.Errorf("Expected Servings '4', got '%s'", normalizedRecipe.Servings)
		}
		if len(normalizedRecipe.Embedding) != 3 {
			t.Errorf("Expected embedding of length 3, got %d", len(normalizedRecipe.Embedding))
		}
	})

	t.Run("LLMError", func(t *testing.T) {
		mockClient := &mockLLMClient{shouldError: true}

		_, err := NormalizeRecipeHTML(ctx, mockClient, post)
		if err == nil {
			t.Fatal("Expected an error from the LLM client, got nil")
		}
		expectedError := "failed to get LLM response: LLM error"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		mockClient := &mockLLMClient{response: "this is not json"}

		_, err := NormalizeRecipeHTML(ctx, mockClient, post)
		if err == nil {
			t.Fatal("Expected an error for invalid JSON, got nil")
		}
		if !strings.HasPrefix(err.Error(), "failed to unmarshal LLM response") {
			t.Errorf("Expected a JSON unmarshaling error, got: %v", err)
		}
	})
}
