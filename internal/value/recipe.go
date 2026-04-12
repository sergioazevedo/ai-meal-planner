package value

import (
	_ "embed"
	"fmt"
)

// Recipe represents a recipe
type Recipe struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Ingredients []string `json:"ingredients"`
	Tags        []string `json:"tags"`
	PrepTime    string   `json:"prep_time"`
	Servings    string   `json:"servings"`
	UpdatedAt   string   `json:"source_updated_at"`
}

// returns a semantic string representation of the recipe
// used for generating embeddings and similarity search.
func (r *Recipe) ToEmbeddingText() string {
	return fmt.Sprintf(
		"Title: %s\nTags: %v\nIngredients: %v\nPrep Time: %s",
		r.Title,
		r.Tags,
		r.Ingredients,
		r.PrepTime,
	)
}
