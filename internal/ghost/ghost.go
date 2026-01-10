package ghost

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ai-meal-planner/internal/config"
)

// Post represents a single recipe post from the Ghost API.
type Post struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

// PostsResponse is the top-level structure of the Ghost API response for posts.
type PostsResponse struct {
	Posts []Post `json:"posts"`
}

// Client is a client for the Ghost Content API.
type Client struct {
	httpClient *http.Client
	config     *config.Config
}

// NewClient creates a new Ghost API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		httpClient: &http.Client{},
		config:     cfg,
	}
}

// FetchRecipes fetches all posts (recipes) from the Ghost API.
func (c *Client) FetchRecipes() ([]Post, error) {
	// Construct the request URL
	url := fmt.Sprintf("%s/ghost/api/v3/content/posts/?key=%s", c.config.GhostURL, c.config.GhostAPIKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	var postsResponse PostsResponse
	if err := json.NewDecoder(resp.Body).Decode(&postsResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return postsResponse.Posts, nil
}