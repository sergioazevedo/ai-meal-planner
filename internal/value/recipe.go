package value

import (
	_ "embed"
	"fmt"
)

// Recipe represents a recipe
type Recipe struct {
	ID          string   `json:"id,omitempty"`
	Title       string   `json:"title,omitempty"`
	Ingredients []string `json:"ingredients,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	PrepTime    string   `json:"prep_time,omitempty"`
	Servings    string   `json:"servings,omitempty"`
	UpdatedAt   string   `json:"source_updated_at,omitempty"`
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
