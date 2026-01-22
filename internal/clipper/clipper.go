package clipper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"

	"github.com/PuerkitoBio/goquery"
)

// Clipper handles fetching and extracting recipes from URLs.
type Clipper struct {
	ghostClient ghost.Client
	textGen     llm.TextGenerator
}

// ExtractedRecipe represents the data structured by the AI.
type ExtractedRecipe struct {
	Title       string   `json:"title"`
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
	PrepTime    string   `json:"prep_time"`
	Servings    string   `json:"servings"`
}

// NewClipper creates a new Clipper instance.
func NewClipper(ghostClient ghost.Client, textGen llm.TextGenerator) *Clipper {
	return &Clipper{
		ghostClient: ghostClient,
		textGen:     textGen,
	}
}

// ClipURL fetches the URL, extracts the recipe using AI, and saves it to Ghost.
func (c *Clipper) ClipURL(ctx context.Context, url string) (*ghost.Post, error) {
	// 1. Fetch and Clean HTML
	content, err := c.fetchAndCleanHTML(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	// 2. Extract Data via Groq
	prompt := fmt.Sprintf(`
You are a recipe extraction expert. Extract the recipe details from the following HTML content.
Return the result strictly as a JSON object with this structure:
{
  "title": "Recipe Title",
  "ingredients": ["item 1", "item 2", ...],
  "steps": ["Step 1 description", "Step 2 description", ...],
  "prep_time": "e.g. 30 mins",
  "servings": "e.g. 4 people"
}

HTML Content:
%s
`, content)

	llmResponse, err := c.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("ai extraction failed: %w", err)
	}

	var extracted ExtractedRecipe
	if err := json.Unmarshal([]byte(llmResponse), &extracted); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w. Response: %s", err, llmResponse)
	}

	// 3. Format as Ghost HTML
	html := c.formatToHTML(extracted, url)

	// 4. Save to Ghost (Published)
	post, err := c.ghostClient.CreatePost(extracted.Title, html, true)
	if err != nil {
		return nil, fmt.Errorf("failed to save to ghost: %w", err)
	}

	return post, nil
}

func (c *Clipper) fetchAndCleanHTML(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Remove noise to save LLM tokens
	doc.Find("script, style, nav, footer, iframe, ads, .ads, #ads").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})

	// Return the text content and some structural tags (p, li, h1-h3)
	return doc.Find("body").Text(), nil
}

func (c *Clipper) formatToHTML(r ExtractedRecipe, sourceURL string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<p><i>Imported from: <a href=\"%s\">%s</a></i></p>", sourceURL, sourceURL))

	sb.WriteString("<h2>Ingredients</h2><ul>")
	for _, ing := range r.Ingredients {
		sb.WriteString(fmt.Sprintf("<li>%s</li>", ing))
	}
	sb.WriteString("</ul>")

	sb.WriteString("<h2>Instructions</h2><ol>")
	for _, step := range r.Steps {
		sb.WriteString(fmt.Sprintf("<li>%s</li>", step))
	}
	sb.WriteString("</ol>")

	sb.WriteString("<hr>")
	sb.WriteString(fmt.Sprintf("<p><strong>Prep Time:</strong> %s | <strong>Servings:</strong> %s</p>", r.PrepTime, r.Servings))

	return sb.String()
}
