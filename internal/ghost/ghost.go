package ghost

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-meal-planner/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// Post represents a single recipe post from the Ghost API.
type Post struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	HTML      string `json:"html"`
	UpdatedAt string `json:"updated_at"`
}

// PostsResponse is the top-level structure of the Ghost API response for posts.
type PostsResponse struct {
	Posts []Post `json:"posts"`
}

// Client is an interface for a Ghost API client (Content & Admin).
type Client interface {
	FetchRecipes() ([]Post, error)
	CreatePost(title, html string, publish bool) (*Post, error)
}

// ghostClient is the concrete implementation of the Ghost API client.
type ghostClient struct {
	httpClient *http.Client
	config     *config.Config
}

// NewClient creates a new Ghost API client.
func NewClient(cfg *config.Config) Client {
	return &ghostClient{
		httpClient: &http.Client{},
		config:     cfg,
	}
}

// FetchRecipes fetches all posts (recipes) from the Ghost Content API.
func (c *ghostClient) FetchRecipes() ([]Post, error) {
	url := fmt.Sprintf("%s/ghost/api/v3/content/posts/?key=%s", c.config.GhostURL, c.config.GhostContentKey)

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
		return nil, fmt.Errorf("content api error: status %d", resp.StatusCode)
	}

	var postsResponse PostsResponse
	if err := json.NewDecoder(resp.Body).Decode(&postsResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return postsResponse.Posts, nil
}

// CreatePost creates a new post using the Ghost Admin API.
func (c *ghostClient) CreatePost(title, html string, publish bool) (*Post, error) {
	token, err := c.createAdminToken()
	if err != nil {
		return nil, fmt.Errorf("failed to create admin token: %w", err)
	}

	status := "draft"
	if publish {
		status = "published"
	}

	newPost := map[string]interface{}{
		"posts": []map[string]interface{}{
			{
				"title":  title,
				"html":   html,
				"status": status,
			},
		},
	}

	body, _ := json.Marshal(newPost)
	url := fmt.Sprintf("%s/ghost/api/v3/admin/posts/?source=html", c.config.GhostURL)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Ghost "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errResp interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("admin api error: status %d, body: %v", resp.StatusCode, errResp)
	}

	var response PostsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Posts) == 0 {
		return nil, fmt.Errorf("no post returned from api")
	}

	return &response.Posts[0], nil
}

// createAdminToken generates a short-lived JWT for the Admin API.
func (c *ghostClient) createAdminToken() (string, error) {
	keyParts := strings.Split(c.config.GhostAdminKey, ":")
	if len(keyParts) != 2 {
		return "", fmt.Errorf("invalid admin key format: expected id:secret")
	}

	id := keyParts[0]
	secretHex := keyParts[1]

	secret, err := hex.DecodeString(secretHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode secret hex: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"aud": "/v3/admin/",
	})
	token.Header["kid"] = id

	return token.SignedString(secret)
}
