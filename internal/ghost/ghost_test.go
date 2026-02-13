package ghost

import (
	"ai-meal-planner/internal/config"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
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
					{"id": "1", "title": "Recipe 1", "html": "<h1>Recipe 1</h1>", "updated_at": "2023-10-27T10:00:00Z"},
					{"id": "2", "title": "Recipe 2", "html": "<h1>Recipe 2</h1>", "updated_at": "2023-10-28T10:00:00Z"}
				],
				"meta": {
					"pagination": {
						"page": 1,
						"limit": 15,
						"pages": 1,
						"total": 2,
						"next": null,
						"prev": null
					}
				}
			}`)
		}))
		defer server.Close()

		// Create a config pointing to the test server
		cfg := &config.Config{
			GhostURL:        server.URL,
			GhostContentKey: "test_key",
		}
		client := NewClient(cfg)

		posts, err := client.FetchRecipes()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(posts) != 2 {
			t.Fatalf("Expected 2 posts, got %d", len(posts))
		}
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := &config.Config{
			GhostURL:        server.URL,
			GhostContentKey: "test_key",
		}
		client := NewClient(cfg)

		_, err := client.FetchRecipes()
		if err == nil {
			t.Fatal("Expected an error for non-200 status code, got nil")
		}
	})
}
