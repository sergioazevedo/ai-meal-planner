package ghost

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"ai-meal-planner/internal/config"
)

func TestFetchRecipes(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Mock Ghost API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check that the key is in the query
			if r.URL.Query().Get("key") != "test_key" {
				t.Errorf("Expected key 'test_key', got '%s'", r.URL.Query().Get("key"))
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{
				"posts": [
					{"id": "1", "title": "Recipe 1", "html": "<h1>Recipe 1</h1>"},
					{"id": "2", "title": "Recipe 2", "html": "<h1>Recipe 2</h1>"}
				]
			}`)
		}))
		defer server.Close()

		// Create a config pointing to the test server
		cfg := &config.Config{
			GhostURL:    server.URL,
			GhostAPIKey: "test_key",
		}
		client := NewClient(cfg)

		posts, err := client.FetchRecipes()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 2 {
			t.Fatalf("Expected 2 posts, got %d", len(posts))
		}
		if posts[0].Title != "Recipe 1" {
			t.Errorf("Expected post 1 title to be 'Recipe 1', got '%s'", posts[0].Title)
		}
		if posts[1].HTML != "<h1>Recipe 2</h1>" {
			t.Errorf("Expected post 2 HTML to be '<h1>Recipe 2</h1>', got '%s'", posts[1].HTML)
		}
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := &config.Config{
			GhostURL:    server.URL,
			GhostAPIKey: "test_key",
		}
		client := NewClient(cfg)

		_, err := client.FetchRecipes()
		if err == nil {
			t.Fatal("Expected an error for non-200 status code, got nil")
		}
		expectedError := "received non-200 status code: 500"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})
}
