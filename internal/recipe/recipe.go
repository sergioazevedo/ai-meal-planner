package recipe

import (
	_ "embed"
	"fmt"
)

type PostData struct {
	ID        string
	Title     string
	UpdatedAt string
	HTML      string
}

// Recipe represents a recipe after being normalized by the LLM.
type Recipe struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions string   `json:"instructions"`
	Tags         []string `json:"tags"`
	PrepTime     string   `json:"prep_time"`
	Servings     string   `json:"servings"`
	UpdatedAt    string   `json:"source_updated_at"`
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

// Contains the normalized recipe and embeding
type NormalizedRecipe struct {
	Recipe
	Embedding []float32 `json:"embedding"`
}
