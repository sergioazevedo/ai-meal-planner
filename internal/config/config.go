package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the configuration for the application.
type Config struct {
	GhostURL        string
	GhostContentKey string
	GhostAdminKey   string
	GeminiAPIKey    string
	GroqAPIKey      string

	// Telegram Config
	TelegramBotToken       string
	TelegramWebhookURL     string
	TelegramAllowedUserIDs []int64
	AdminTelegramID        int64

	MetricsDBPath     string
	RecipeStoragePath string
	
			// Defaults for Planning
			DefaultAdults           int
			DefaultChildren         int
			DefaultChildrenAges     []int
			DefaultCookingFrequency int
		}	
	// NewFromEnv creates a new Config object from environment variables.
	func NewFromEnv() (*Config, error) {
		ghostURL := os.Getenv("GHOST_API_URL")
		if ghostURL == "" {
			return nil, fmt.Errorf("GHOST_API_URL environment variable not set")
		}
	
		ghostContentKey := os.Getenv("GHOST_CONTENT_API_KEY")
		if ghostContentKey == "" {
			return nil, fmt.Errorf("GHOST_CONTENT_API_KEY environment variable not set")
		}
	
		ghostAdminKey := os.Getenv("GHOST_ADMIN_API_KEY")
		if ghostAdminKey == "" {
			// Fallback to content key if only one is provided
			ghostAdminKey = ghostContentKey
		}
	
		geminiAPIKey := os.Getenv("GEMINI_API_KEY")
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
		
		var allowedIDs []int64
		idsStr := os.Getenv("TELEGRAM_ALLOWED_USER_IDS")
		if idsStr == "" {
			// Backward compatibility for the old single ID variable
			idsStr = os.Getenv("TELEGRAM_ALLOW_USER_ID")
		}
	
		if idsStr != "" {
			parts := strings.Split(idsStr, ",")
			for _, p := range parts {
				var id int64
				if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &id); err == nil {
					allowedIDs = append(allowedIDs, id)
				}
			}
		}
	
	recipeStoragePath := os.Getenv("RECIPE_STORAGE_PATH")
	if recipeStoragePath == "" {
		recipeStoragePath = "data/recipes"
	}

	metricsDBPath := os.Getenv("METRICS_DB_PATH")
	if metricsDBPath == "" {
		metricsDBPath = "data/db/metrics.db"
	}

	var adminID int64
	if idStr := os.Getenv("ADMIN_TELEGRAM_ID"); idStr != "" {
		fmt.Sscanf(idStr, "%d", &adminID)
	}

	// Default Planning Values
	defaultAdults := 2
	if val := os.Getenv("DEFAULT_ADULTS"); val != "" {
		fmt.Sscanf(val, "%d", &defaultAdults)
	}

	defaultChildren := 1
	if val := os.Getenv("DEFAULT_CHILDREN"); val != "" {
		fmt.Sscanf(val, "%d", &defaultChildren)
	}

	var defaultAges []int
	if val := os.Getenv("DEFAULT_CHILDREN_AGES"); val != "" {
		parts := strings.Split(val, ",")
		for _, p := range parts {
			var age int
			if _, err := fmt.Sscanf(strings.TrimSpace(p), "%d", &age); err == nil {
				defaultAges = append(defaultAges, age)
			}
		}
	} else if defaultChildren > 0 {
		// Default to 5-year olds if children are present but no ages given
		for i := 0; i < defaultChildren; i++ {
			defaultAges = append(defaultAges, 5)
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
		MetricsDBPath:           metricsDBPath,
		RecipeStoragePath:       recipeStoragePath,
		DefaultAdults:           defaultAdults,
		DefaultChildren:         defaultChildren,
		DefaultChildrenAges:     defaultAges,
		DefaultCookingFrequency: defaultFreq,
	}, nil
}
