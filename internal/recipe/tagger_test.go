package recipe

import (
	"context"
	"os"
	"slices"
	"testing"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/llm/llmtest"
	"ai-meal-planner/internal/value"
)

func TestTaggerRunReturnsCompleteBilingualPairs(t *testing.T) {
	textGen := &llmtest.MockTextGenerator{Response: `{
		"tags": [
			{"pt-BR": " Salmão ", "en": " Salmon "},
			{"pt-BR": "Brócolis", "en": "Broccoli"},
			{"pt-BR": "peixe", "en": "fish"}
		]
	}`}
	tagger := NewTagger(textGen)

	result, err := tagger.Run(context.Background(), salmonRecipe(), []string{"Air Fryer"})
	if err != nil {
		t.Fatalf("Tagger.Run() error = %v", err)
	}

	want := []string{"salmão", "salmon", "brócolis", "broccoli", "peixe", "fish"}
	if !slices.Equal(result.Tags, want) {
		t.Fatalf("tags = %#v, want %#v", result.Tags, want)
	}
	if result.Meta.AgentName != "Tagger" {
		t.Fatalf("agent name = %q, want Tagger", result.Meta.AgentName)
	}
}

func TestTaggerRunRepairsIncompletePair(t *testing.T) {
	textGen := &llmtest.MockTextGenerator{ResponseChain: []llm.ContentResponse{
		{Message: llm.Message{Role: "assistant", Content: `{"tags":[{"pt-BR":"salmão","en":""}]}`}},
		{Message: llm.Message{Role: "assistant", Content: `{"tags":[{"pt-BR":"salmão","en":"salmon"}]}`}},
	}}

	result, err := NewTagger(textGen).Run(context.Background(), salmonRecipe(), nil)
	if err != nil {
		t.Fatalf("Tagger.Run() error = %v", err)
	}
	if !slices.Equal(result.Tags, []string{"salmão", "salmon"}) {
		t.Fatalf("tags = %#v", result.Tags)
	}
}

// TestTagger_LiveEval guards the real Groq model and prompt against the
// bilingual salmon-tag regression seen in production.
func TestTagger_LiveEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live tagger eval in short mode")
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("GROQ_API_KEY must be configured for the CI tagger eval")
		}
		t.Skip("skipping live tagger eval: GROQ_API_KEY is not configured")
	}
	cfg := &config.Config{GroqAPIKey: apiKey}

	client := llm.NewGroqClient(cfg, llm.ModelTagger, 0.0)
	result, err := NewTagger(client).Run(context.Background(), salmonRecipe(), []string{"air fryer", "jantar"})
	if err != nil {
		t.Fatalf("live tagger failed: %v", err)
	}

	for _, pair := range [][2]string{
		{"salmão", "salmon"},
		{"brócolis", "broccoli"},
		{"peixe", "fish"},
		{"fritadeira sem óleo", "air fryer"},
		{"jantar", "dinner"},
	} {
		assertOrderedTagPair(t, result.Tags, pair[0], pair[1])
	}
	for _, forbidden := range []string{
		"vegetariano", "vegetarian", "vegano", "vegan",
		"baixo carboidrato", "low carb", "sem glúten", "gluten free",
		"asador de ar",
	} {
		if slices.Contains(result.Tags, forbidden) {
			t.Errorf("QUALITY FAIL: tagger returned forbidden tag %q in %#v", forbidden, result.Tags)
		}
	}

	t.Logf("tagger eval passed with tags: %v", result.Tags)
}

func assertOrderedTagPair(t *testing.T, tags []string, portuguese, english string) {
	t.Helper()
	for i := 0; i+1 < len(tags); i++ {
		if tags[i] == portuguese && tags[i+1] == english {
			return
		}
	}
	t.Errorf("QUALITY FAIL: missing ordered pt-BR/en pair %q/%q in %#v", portuguese, english, tags)
}

func salmonRecipe() value.Recipe {
	return value.Recipe{
		Title: "Salmão com brócolis",
		Ingredients: []string{
			"1 tranche de salmão com pele (cerca de 200 g)",
			"5 ramos de brócolis (floretes com talo)",
			"2 colheres (sopa) de shoyu (molho de soja)",
			"1 colher (sopa) de mel",
			"1 colher (chá) de vinagre de arroz",
		},
	}
}
