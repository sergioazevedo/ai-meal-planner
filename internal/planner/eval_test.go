package planner

import (
	"context"
	"os"
	"testing"
	"time"

	"ai-meal-planner/internal/config"
)

const plannerLiveEvalTimeout = 2 * time.Minute

func liveEvalContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), plannerLiveEvalTimeout)
	t.Cleanup(cancel)
	return ctx
}

func liveEvalConfig(t *testing.T) *config.Config {
	t.Helper()

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("GROQ_API_KEY must be configured for planner evals in CI")
		}
		t.Skip("skipping live planner eval: GROQ_API_KEY is not configured")
	}

	return &config.Config{GroqAPIKey: apiKey}
}

func liveEvalModel(envKey, fallback string) string {
	if model := os.Getenv(envKey); model != "" {
		return model
	}
	return fallback
}
