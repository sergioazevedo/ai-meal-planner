package config

import (
	"fmt"
	"os"
)

// Config holds the configuration for the application.
type Config struct {
	GhostURL    string
	GhostAPIKey string
	GeminiAPIKey string // New field for Gemini API Key
	GroqAPIKey   string
	
	// Telegram Config
	TelegramBotToken   string
	TelegramWebhookURL string
	TelegramAllowUserID int64
}

// NewFromEnv creates a new Config object from environment variables.
func NewFromEnv() (*Config, error) {
	ghostURL := os.Getenv("GHOST_API_URL")
	if ghostURL == "" {
		return nil, fmt.Errorf("GHOST_API_URL environment variable not set")
	}

	ghostAPIKey := os.Getenv("GHOST_CONTENT_API_KEY")
	if ghostAPIKey == "" {
		return nil, fmt.Errorf("GHOST_CONTENT_API_KEY environment variable not set")
	}

	geminiAPIKey := os.Getenv("GEMINI_API_KEY") // Load Gemini API Key
	if geminiAPIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	groqAPIKey := os.Getenv("GROQ_API_KEY")
	if groqAPIKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY environment variable not set")
	}

	// Telegram Config (Optional for CLI, required for Bot)
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramWebhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")
	telegramAllowUserIDStr := os.Getenv("TELEGRAM_ALLOW_USER_ID")
	var telegramAllowUserID int64
	if telegramAllowUserIDStr != "" {
		fmt.Sscanf(telegramAllowUserIDStr, "%d", &telegramAllowUserID)
	}

	return &Config{
		GhostURL:           ghostURL,
		GhostAPIKey:        ghostAPIKey,
		GeminiAPIKey:       geminiAPIKey,
		GroqAPIKey:         groqAPIKey,
		TelegramBotToken:   telegramBotToken,
		TelegramWebhookURL: telegramWebhookURL,
		TelegramAllowUserID: telegramAllowUserID,
	}, nil
}