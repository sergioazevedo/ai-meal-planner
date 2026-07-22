package recipe

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/shared"
	"ai-meal-planner/internal/value"
)

//go:embed tagger_prompt.md
var taggerPrompt string

const maxTaggerAttempts = 2

type TagPair struct {
	Portuguese string `json:"pt-BR"`
	English    string `json:"en"`
}

type taggerResponse struct {
	Tags []TagPair `json:"tags"`
}

type TaggerResult struct {
	Tags []string
	Meta shared.AgentMeta
}

// Tagger enriches an already-normalized recipe with bilingual tags.
type Tagger struct {
	textGen llm.TextGenerator
}

func NewTagger(textGen llm.TextGenerator) *Tagger {
	return &Tagger{textGen: textGen}
}

func (t *Tagger) Run(ctx context.Context, rec value.Recipe, sourceTags []string) (TaggerResult, error) {
	if t == nil || t.textGen == nil {
		return TaggerResult{}, fmt.Errorf("tagger text generator is not configured")
	}

	prompt, err := buildTaggerPrompt(rec, sourceTags)
	if err != nil {
		return TaggerResult{}, err
	}

	start := time.Now()
	conversation := llm.Conversation{{Role: "user", Content: prompt}}
	var usage shared.TokenUsage
	var lastErr error

	for attempt := 0; attempt < maxTaggerAttempts; attempt++ {
		resp, err := t.textGen.GenerateContent(ctx, conversation, llm.NoTools)
		if err != nil {
			return TaggerResult{}, fmt.Errorf("failed to get tagger response: %w", err)
		}
		usage = addTokenUsage(usage, resp.Usage)

		tags, err := parseTaggerResponse(resp.Message.Content)
		if err == nil {
			return TaggerResult{
				Tags: tags,
				Meta: shared.AgentMeta{AgentName: "Tagger", Usage: usage, Latency: time.Since(start)},
			}, nil
		}

		lastErr = err
		conversation = conversation.Add(resp.Message)
		conversation = conversation.Add(llm.Message{
			Role: "user",
			Content: "The tag response was invalid: " + err.Error() +
				". Return the corrected raw JSON object with complete pt-BR/en translation pairs only.",
		})
	}

	return TaggerResult{}, fmt.Errorf("invalid tagger response after %d attempts: %w", maxTaggerAttempts, lastErr)
}

func buildTaggerPrompt(rec value.Recipe, sourceTags []string) (string, error) {
	ingredientsJSON, err := json.Marshal(rec.Ingredients)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tagger ingredients: %w", err)
	}
	sourceTagsJSON, err := json.Marshal(sourceTags)
	if err != nil {
		return "", fmt.Errorf("failed to marshal source tags: %w", err)
	}

	tmpl, err := template.New("tagger").Parse(taggerPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to parse tagger prompt: %w", err)
	}

	data := struct {
		Title           string
		IngredientsJSON string
		SourceTagsJSON  string
	}{rec.Title, string(ingredientsJSON), string(sourceTagsJSON)}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to build tagger prompt: %w", err)
	}
	return buf.String(), nil
}

func parseTaggerResponse(content string) ([]string, error) {
	var response taggerResponse
	decoder := json.NewDecoder(strings.NewReader(llm.CleanJSON(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode tags: %w", err)
	}
	if len(response.Tags) == 0 {
		return nil, fmt.Errorf("tags must contain at least one translation pair")
	}

	seen := make(map[string]struct{}, len(response.Tags)*2)
	tags := make([]string, 0, len(response.Tags)*2)
	for i, pair := range response.Tags {
		pt := strings.ToLower(strings.TrimSpace(pair.Portuguese))
		en := strings.ToLower(strings.TrimSpace(pair.English))
		if pt == "" || en == "" {
			return nil, fmt.Errorf("tag pair %d must contain both pt-BR and en", i)
		}
		for _, tag := range []string{pt, en} {
			if _, exists := seen[tag]; exists {
				continue
			}
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func addTokenUsage(total, current shared.TokenUsage) shared.TokenUsage {
	total.PromptTokens += current.PromptTokens
	total.CompletionTokens += current.CompletionTokens
	total.TotalTokens += current.TotalTokens
	total.Model = current.Model
	return total
}
