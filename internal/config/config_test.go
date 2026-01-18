package config

import (
	"os"
	"testing"
)

func TestNewFromEnv(t *testing.T) {
	// Helper function to set environment variables for a test
	setEnv := func(key, value string) {
		t.Helper()
		t.Setenv(key, value)
	}

	t.Run("Success", func(t *testing.T) {
		setEnv("GHOST_API_URL", "http://ghost.test")
		setEnv("GHOST_CONTENT_API_KEY", "ghost_key")
		setEnv("GEMINI_API_KEY", "gemini_key")
		setEnv("GROQ_API_KEY", "groq_key")

		cfg, err := NewFromEnv()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if cfg.GhostURL != "http://ghost.test" {
			t.Errorf("Expected GhostURL to be 'http://ghost.test', got '%s'", cfg.GhostURL)
		}
		if cfg.GhostAPIKey != "ghost_key" {
			t.Errorf("Expected GhostAPIKey to be 'ghost_key', got '%s'", cfg.GhostAPIKey)
		}
		if cfg.GeminiAPIKey != "gemini_key" {
			t.Errorf("Expected GeminiAPIKey to be 'gemini_key', got '%s'", cfg.GeminiAPIKey)
		}
		if cfg.GroqAPIKey != "groq_key" {
			t.Errorf("Expected GroqAPIKey to be 'groq_key', got '%s'", cfg.GroqAPIKey)
		}
	})

	t.Run("MissingGhostURL", func(t *testing.T) {
		setEnv("GHOST_CONTENT_API_KEY", "ghost_key")
		setEnv("GEMINI_API_KEY", "gemini_key")
		setEnv("GROQ_API_KEY", "groq_key")

		// Unset GHOST_API_URL specifically for this test
		os.Unsetenv("GHOST_API_URL")

		_, err := NewFromEnv()
		if err == nil {
			t.Fatal("Expected an error for missing GHOST_API_URL, got nil")
		}
		expectedError := "GHOST_API_URL environment variable not set"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("MissingGhostAPIKey", func(t *testing.T) {
		setEnv("GHOST_API_URL", "http://ghost.test")
		setEnv("GEMINI_API_KEY", "gemini_key")
		setEnv("GROQ_API_KEY", "groq_key")

		os.Unsetenv("GHOST_CONTENT_API_KEY")

		_, err := NewFromEnv()
		if err == nil {
			t.Fatal("Expected an error for missing GHOST_CONTENT_API_KEY, got nil")
		}
		expectedError := "GHOST_CONTENT_API_KEY environment variable not set"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("MissingGeminiAPIKey", func(t *testing.T) {
		setEnv("GHOST_API_URL", "http://ghost.test")
		setEnv("GHOST_CONTENT_API_KEY", "ghost_key")
		setEnv("GROQ_API_KEY", "groq_key")

		os.Unsetenv("GEMINI_API_KEY")

		_, err := NewFromEnv()
		if err == nil {
			t.Fatal("Expected an error for missing GEMINI_API_KEY, got nil")
		}
		expectedError := "GEMINI_API_KEY environment variable not set"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("MissingGroqAPIKey", func(t *testing.T) {
		setEnv("GHOST_API_URL", "http://ghost.test")
		setEnv("GHOST_CONTENT_API_KEY", "ghost_key")
		setEnv("GEMINI_API_KEY", "gemini_key")

		os.Unsetenv("GROQ_API_KEY")

		_, err := NewFromEnv()
		if err == nil {
			t.Fatal("Expected an error for missing GROQ_API_KEY, got nil")
		}
		expectedError := "GROQ_API_KEY environment variable not set"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})
}
