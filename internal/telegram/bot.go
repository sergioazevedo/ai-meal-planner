package telegram

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/planner"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram API and our Meal Planner.
type Bot struct {
	api     *tgbotapi.BotAPI
	planner *planner.Planner
	cfg     *config.Config
}

// NewBot initializes the Telegram Bot and sets the Webhook.
func NewBot(cfg *config.Config, planner *planner.Planner) (*Bot, error) {
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
		api:     bot,
		planner: planner,
		cfg:     cfg,
	},
	nil
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

	if update.Message.From.ID != b.cfg.TelegramAllowUserID {
		log.Printf("⚠️ Unauthorized access attempt from UserID: %d (@%s)", update.Message.From.ID, update.Message.From.UserName)
		return
	}

	go b.processMessage(update.Message)
}

func (b *Bot) processMessage(msg *tgbotapi.Message) {
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, "🧑‍🍳 *Thinking...* \n(Analyzing recipes and generating your plan)")
	replyMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(replyMsg)
	if err != nil {
		log.Printf("Failed to send initial reply: %v", err)
		return
	}

	ctx := context.Background()
	log.Printf("Generating plan for request: %s", msg.Text)
	plan, err := b.planner.GeneratePlan(ctx, msg.Text)

	var finalText string
	if err != nil {
		log.Printf("Error generating plan: %v", err)
		finalText = fmt.Sprintf("❌ *Error generating plan:*
`%v`", err)
	} else {
		finalText = formatPlanMarkdown(plan)
	}

	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
	edit.ParseMode = "Markdown"
	
	if _, err := b.api.Send(edit); err != nil {
		log.Printf("Failed to edit message: %v", err)
		newMsg := tgbotapi.NewMessage(msg.Chat.ID, finalText)
		newMsg.ParseMode = "Markdown"
		b.api.Send(newMsg)
	}
}

func formatPlanMarkdown(plan *planner.MealPlan) string {
	var sb strings.Builder
	sb.WriteString("📅 *Weekly Meal Plan*\n\n")

	for _, dp := range plan.Plan {
		sb.WriteString(fmt.Sprintf("*%s*: %s\n", dp.Day, dp.RecipeTitle))
		if dp.Note != "" {
			sb.WriteString(fmt.Sprintf("_%s_\n", dp.Note))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("🛒 *Shopping List*\n")
	for _, item := range plan.ShoppingList {
		sb.WriteString(fmt.Sprintf("• %s\n", item))
	}

	sb.WriteString(fmt.Sprintf("\n⏱ *Total Prep:* %s", plan.TotalPrep))
	return sb.String()
}
