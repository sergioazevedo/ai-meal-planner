package clipper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-meal-planner/internal/ghost"
)

// --- Mocks ---
type MockGhostClient struct {
	CreatedPost *ghost.Post
	ShouldError bool
}

func (m *MockGhostClient) FetchRecipes() ([]ghost.Post, error) {
	return nil, nil
}

func (m *MockGhostClient) CreatePost(title, html string, publish bool) (*ghost.Post, error) {
	if m.ShouldError {
		return nil, fmt.Errorf("mock error")
	}
	m.CreatedPost = &ghost.Post{ID: "123", Title: title, HTML: html}
	return m.CreatedPost, nil
}

type MockTextGenerator struct {
	Response    string
	ShouldError bool
}

func (m *MockTextGenerator) GenerateContent(ctx context.Context, prompt string) (string, error) {
	if m.ShouldError {
		return "", fmt.Errorf("mock ai error")
	}
	return m.Response, nil
}

// --- Tests ---

func TestFetchAndCleanHTML(t *testing.T) {
	// 1. Setup a test server serving dirty HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
			<head><script>alert('bad');</script></head>
			<body>
				<h1>Tasty Recipe</h1>
				<div class="ads">Buy stuff!</div>
				<p>Mix flour and water.</p>
				<script>more_bad_stuff()</script>
				<footer>Copyright 2024</footer>
			</body>
		</html>`
		w.Write([]byte(html))
	}))
	defer ts.Close()

	// 2. Initialize Clipper (deps don't matter for this private method test, but we need the struct)
	c := NewClipper(&MockGhostClient{}, &MockTextGenerator{})

	// 3. Run the private method (using export_test.go trick or just testing public ClipURL if preferred)
	// Since go doesn't allow testing private methods easily from external test package, 
	// we are in package clipper, so we can access it.
	cleanText, err := c.fetchAndCleanHTML(ts.URL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// 4. Assertions
	if strings.Contains(cleanText, "alert('bad')") {
		t.Error("Failed to remove <script> tags")
	}
	if strings.Contains(cleanText, "Buy stuff!") {
		t.Error("Failed to remove .ads class")
	}
	if strings.Contains(cleanText, "Copyright 2024") {
		t.Error("Failed to remove <footer>")
	}
	if !strings.Contains(cleanText, "Tasty Recipe") {
		t.Error("Expected to find 'Tasty Recipe'")
	}
	if !strings.Contains(cleanText, "Mix flour and water.") {
		t.Error("Expected to find body content")
	}
}

func TestFormatToHTML(t *testing.T) {
	c := NewClipper(nil, nil)
	
	recipe := ExtractedRecipe{
		Title:       "Pancakes",
		Ingredients: []string{"Flour", "Milk"},
		Steps:       []string{"Mix", "Fry"},
		PrepTime:    "10m",
		Servings:    "2",
	}
	
	html := c.formatToHTML(recipe, "http://test.com")

	expectedSubstrings := []string{
		"Imported from: <a href=\"http://test.com\">http://test.com</a>",
		"<li>Flour</li>",
		"<li>Mix</li>",
		"<strong>Prep Time:</strong> 10m",
	}

	for _, sub := range expectedSubstrings {
		if !strings.Contains(html, sub) {
			t.Errorf("Expected HTML to contain '%s'", sub)
		}
	}
}

func TestClipURL_Success(t *testing.T) {
	// Mock AI Response
	aiResponse := `{"title": "Mock Pie", "ingredients": ["Apple"], "steps": ["Bake"], "prep_time": "1h", "servings": "8"}`

	mockGhost := &MockGhostClient{}
	mockAI := &MockTextGenerator{Response: aiResponse}
	c := NewClipper(mockGhost, mockAI)

	// Mock Server for the URL fetch
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Some Content</body></html>"))
	}))
	defer ts.Close()

	post, err := c.ClipURL(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("ClipURL failed: %v", err)
	}

	if post.Title != "Mock Pie" {
		t.Errorf("Expected title 'Mock Pie', got '%s'", post.Title)
	}
	if mockGhost.CreatedPost == nil {
		t.Fatal("Expected Ghost CreatePost to be called")
	}
	if !strings.Contains(mockGhost.CreatedPost.HTML, "Apple") {
		t.Error("Expected HTML content to contain extracted ingredients")
	}
}
