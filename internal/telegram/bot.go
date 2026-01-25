package telegram

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram API, Meal Planner, and Clipper.
type Bot struct {
	api         *tgbotapi.BotAPI
	planner     *planner.Planner
	clipper     *clipper.Clipper
	recipeStore *storage.RecipeStore
	textGen     llm.TextGenerator
	embedGen    llm.EmbeddingGenerator
	cfg         *config.Config
}

// NewBot initializes the Telegram Bot and sets the Webhook.
func NewBot(
	cfg *config.Config,
	planner *planner.Planner,
	clipper *clipper.Clipper,
	store *storage.RecipeStore,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.TelegramBotToken)
	if err != nil {
		return nil, fmt.Errorf("failed to init telegram api: %w", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	webhookURL := cfg.TelegramWebhookURL
	wh, _ := tgbotapi.NewWebhook(webhookURL)
	resp, err := bot.Request(wh)
	if err != nil {
		return nil, fmt.Errorf("failed to set webhook to %s: %w", webhookURL, err)
	}
	log.Printf("Webhook set response: %s", resp.Description)

	return &Bot{
		api:         bot,
		planner:     planner,
		clipper:     clipper,
		recipeStore: store,
		textGen:     textGen,
		embedGen:    embedGen,
		cfg:         cfg,
	}, nil
}

// RegisterHandlers registers the webhook handler with the default HTTP mux.

func (b *Bot) RegisterHandlers() {
	http.HandleFunc("/webhook", b.handleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

func (b *Bot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	update, err := b.api.HandleUpdate(r)
	if err != nil {
		log.Printf("Error parsing update: %v", err)
		return
	}

	if update.Message == nil {
		return
	}

	isAllowed := false
	for _, id := range b.cfg.TelegramAllowedUserIDs {
		if update.Message.From.ID == id {
			isAllowed = true
			break
		}
	}

	if !isAllowed {
		log.Printf("⚠️ Unauthorized access attempt from UserID: %d (@%s)", update.Message.From.ID, update.Message.From.UserName)
		return
	}

	go b.processMessage(update.Message)
}

func (b *Bot) processMessage(msg *tgbotapi.Message) {
	// 1. Detect if it's a URL (Clipper mode) or a request (Planner mode)
	isURL := strings.HasPrefix(msg.Text, "http://") || strings.HasPrefix(msg.Text, "https://")

	var statusText string
	if isURL {
		statusText = "✂️ *Clipping recipe...* \n(Extracting and saving to your blog)"
	} else {
		statusText = "🧑‍🍳 *Thinking...* \n(Analyzing recipes and generating your plan)"
	}

	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, statusText)
	replyMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(replyMsg)
	if err != nil {
		log.Printf("Failed to send initial reply: %v", err)
		return
	}

	ctx := context.Background()

	if isURL {
		// --- Clipper Flow ---
		post, err := b.clipper.ClipURL(ctx, msg.Text)
		var finalText string
		if err != nil {
			log.Printf("Error clipping recipe: %v", err)
			safeErr := strings.ReplaceAll(err.Error(), "`", "'")
			finalText = fmt.Sprintf("❌ *Error clipping recipe:*\n```\n%v\n```", safeErr)
		} else {
			finalText = fmt.Sprintf("✅ *Recipe Saved!*\n\n*Title:* %s\n*URL:* %s/%s", post.Title, b.cfg.GhostURL, post.ID)
			// Trigger background ingestion so it becomes searchable for future plans
			go b.ingestClippedPost(*post)
		}
		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
	} else {
		// --- Planner Flow ---
		log.Printf("Generating plan for request: %s", msg.Text)

		// Simple heuristic to extract context from natural language
		// In a production app, we might use a dedicated LLM call to extract these parameters.
		// For now, we'll use defaults and allow simple overrides in text.
		pCtx := planner.PlanningContext{
			Adults:           b.cfg.DefaultAdults,
			Children:         b.cfg.DefaultChildren,
			ChildrenAges:     b.cfg.DefaultChildrenAges,
			CookingFrequency: b.cfg.DefaultCookingFrequency,
		}

		// Basic extraction for demo purposes
		if strings.Contains(strings.ToLower(msg.Text), "adults") {
			fmt.Sscanf(msg.Text, "%d adults", &pCtx.Adults)
		}

		plan, err := b.planner.GeneratePlan(ctx, msg.Text, pCtx)

		if err != nil {
			log.Printf("Error generating plan: %v", err)
			safeErr := strings.ReplaceAll(err.Error(), "`", "'")
			finalText := fmt.Sprintf("❌ *Error generating plan:*\n```\n%v\n```", safeErr)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
		} else {
			planText, shoppingListText := formatPlanMarkdownParts(plan)

			// Edit first message with the Plan
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, planText)
			edit.ParseMode = "Markdown"
			b.api.Send(edit)

			// Send second message with the Shopping List
			shoppingMsg := tgbotapi.NewMessage(msg.Chat.ID, shoppingListText)
			shoppingMsg.ParseMode = "Markdown"
			b.api.Send(shoppingMsg)
		}
	}
}

func formatPlanMarkdownParts(plan *planner.MealPlan) (string, string) {
	var pb strings.Builder
	pb.WriteString("📅 *Weekly Meal Plan*\n\n")

	for _, dp := range plan.Plan {
		pb.WriteString(fmt.Sprintf("*%s*: %s", dp.Day, dp.RecipeTitle))
		if dp.PrepTime != "" {
			pb.WriteString(fmt.Sprintf(" (%s)", dp.PrepTime))
		}
		pb.WriteString("\n")
		if dp.Note != "" {
			pb.WriteString(fmt.Sprintf("_%s_\n", dp.Note))
		}
		pb.WriteString("\n")
	}

	var sb strings.Builder
	sb.WriteString("🛒 *Shopping List*\n\n")
	for _, item := range plan.ShoppingList {
		sb.WriteString(fmt.Sprintf("• %s\n", item))
	}

	return pb.String(), sb.String()
}

// ingestClippedPost performs normalization and storage in the background.
func (b *Bot) ingestClippedPost(post ghost.Post) {
	log.Printf("Background: Ingesting clipped recipe '%s'...", post.Title)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	normalizedRecipe, err := recipe.NormalizeRecipeHTML(ctx, b.textGen, b.embedGen, post)
	if err != nil {
		log.Printf("Background Error: Failed to normalize '%s': %v", post.Title, err)
		return
	}

	if err := b.recipeStore.Save(post.ID, post.UpdatedAt, *normalizedRecipe); err != nil {
		log.Printf("Background Error: Failed to save '%s' to store: %v", post.Title, err)
		return
	}

	log.Printf("Background Success: Recipe '%s' is now indexed and searchable.", post.Title)
}
