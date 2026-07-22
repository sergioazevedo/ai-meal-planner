package recipe

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
)

func TestExtractor_LiveEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live extractor eval in short mode")
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("GROQ_API_KEY must be configured for the CI extractor eval")
		}
		t.Skip("skipping live extractor eval: GROQ_API_KEY is not configured")
	}
	model := os.Getenv("GROQ_NORMALIZER_MODEL")
	if model == "" {
		model = llm.ModelNormalizer
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	extractor := NewExtractor(llm.NewGroqClient(&config.Config{GroqAPIKey: apiKey}, model, 0.1), nil, nil)
	result, err := extractor.ExtractRecipe(ctx, PostData{
		ID:    "salmon",
		Title: "Salmão grelhado com brócolis",
		HTML: `<h1>Salmão grelhado</h1>
<p>Serve 2 pessoas. Tempo de preparo: 25 minutos.</p>
<h2>Ingredientes</h2><p>2 filés de salmão, 1 colher de azeite, sal.</p>
<h2>Acompanhamento</h2><p>200g de brócolis cozidos.</p>`,
	})
	if err != nil {
		t.Fatalf("live extractor failed: %v", err)
	}

	recipe := result.Recipe
	if !strings.Contains(strings.ToLower(recipe.Title), "salmão") || strings.Contains(strings.ToLower(recipe.Title), "brócolis") {
		t.Errorf("QUALITY FAIL: main title was not separated from the side dish: %q", recipe.Title)
	}
	if len(recipe.SideDishes) != 1 || !strings.Contains(strings.ToLower(recipe.SideDishes[0]), "brócolis") {
		t.Errorf("QUALITY FAIL: broccoli side dish missing from %#v", recipe.SideDishes)
	}
	if len(recipe.Ingredients) < 4 || recipe.PrepTime == "" || recipe.Servings == "" {
		t.Errorf("STRUCTURE FAIL: incomplete extracted recipe: %#v", recipe)
	}
	t.Logf("extractor returned: title=%q sides=%v ingredients=%v", recipe.Title, recipe.SideDishes, recipe.Ingredients)
}
