package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	GhostURL                string
	GhostContentKey         string
	GhostAdminKey           string
	GeminiAPIKey            string
	GroqAPIKey              string
	TelegramBotToken        string
	TelegramWebhookURL      string
	TelegramAllowedUserIDs  []int64
	AdminTelegramID         int64
	DatabasePath            string
	DefaultAdults           int
	DefaultChildren         int
	DefaultChildrenAges     []int
	DefaultCookingFrequency int
}

// NewFromEnv creates a new Config from environment variables.
func NewFromEnv() (*Config, error) {
	ghostURL := os.Getenv("GHOST_URL")
	ghostContentKey := os.Getenv("GHOST_CONTENT_KEY")
	ghostAdminKey := os.Getenv("GHOST_ADMIN_KEY")
	geminiAPIKey := os.Getenv("GEMINI_API_KEY")
	groqAPIKey := os.Getenv("GROQ_API_KEY")
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramWebhookURL := os.Getenv("TELEGRAM_WEBHOOK_URL")
	databasePath := os.Getenv("DATABASE_PATH")
	if databasePath == "" {
		databasePath = "data/db/meal_planner.db"
	}

	allowedIDsStr := os.Getenv("TELEGRAM_ALLOWED_USER_IDS")
	var allowedIDs []int64
	if allowedIDsStr != "" {
		for _, idStr := range strings.Split(allowedIDsStr, ",") {
			var id int64
			if _, err := fmt.Sscanf(strings.TrimSpace(idStr), "%d", &id); err == nil {
				allowedIDs = append(allowedIDs, id)
			}
		}
	}

	var adminID int64
	if val := os.Getenv("ADMIN_TELEGRAM_ID"); val != "" {
		fmt.Sscanf(val, "%d", &adminID)
	}

	defaultAdults := 2
	if val := os.Getenv("DEFAULT_ADULTS"); val != "" {
		fmt.Sscanf(val, "%d", &defaultAdults)
	}

	defaultChildren := 0
	if val := os.Getenv("DEFAULT_CHILDREN"); val != "" {
		fmt.Sscanf(val, "%d", &defaultChildren)
	}

	var defaultAges []int
	if val := os.Getenv("DEFAULT_CHILDREN_AGES"); val != "" {
		for _, ageStr := range strings.Split(val, ",") {
			var age int
			if _, err := fmt.Sscanf(strings.TrimSpace(ageStr), "%d", &age); err == nil {
				defaultAges = append(defaultAges, age)
			}
		}
	}

	defaultFreq := 5
	if val := os.Getenv("DEFAULT_COOKING_FREQUENCY"); val != "" {
		fmt.Sscanf(val, "%d", &defaultFreq)
	}

	return &Config{
		GhostURL:                ghostURL,
		GhostContentKey:         ghostContentKey,
		GhostAdminKey:           ghostAdminKey,
		GeminiAPIKey:            geminiAPIKey,
		GroqAPIKey:              groqAPIKey,
		TelegramBotToken:        telegramBotToken,
		TelegramWebhookURL:      telegramWebhookURL,
		TelegramAllowedUserIDs:  allowedIDs,
		AdminTelegramID:         adminID,
		DatabasePath:            databasePath,
		DefaultAdults:           defaultAdults,
		DefaultChildren:         defaultChildren,
		DefaultChildrenAges:     defaultAges,
		DefaultCookingFrequency: defaultFreq,
	}, nil
}
