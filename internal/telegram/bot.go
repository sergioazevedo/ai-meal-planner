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
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps the Telegram API, Meal Planner, and Clipper.
type Bot struct {
	api          *tgbotapi.BotAPI
	planner      *planner.Planner
	clipper      *clipper.Clipper
	metricsStore *metrics.Store
	textGen      llm.TextGenerator
	embedGen     llm.EmbeddingGenerator
	cfg          *config.Config

	// New repositories
	planRepo   *planner.PlanRepository
	recipeRepo *recipe.Repository
	vectorRepo *llm.VectorRepository
}

// NewBot initializes the Telegram Bot and sets the Webhook.
func NewBot(
	cfg *config.Config,
	planner *planner.Planner,
	clipper *clipper.Clipper,
	metricsStore *metrics.Store,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	planRepo *planner.PlanRepository, // New parameter
	recipeRepo *recipe.Repository,    // New parameter
	vectorRepo *llm.VectorRepository, // New parameter
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
		api:          bot,
		planner:      planner,
		clipper:      clipper,
		metricsStore: metricsStore,
		textGen:      textGen,
		embedGen:     embedGen,
		cfg:          cfg,
		planRepo:     planRepo,
		recipeRepo:   recipeRepo,
		vectorRepo:   vectorRepo,
	}, nil
}

// RegisterHandlers registers the webhook handler with the default HTTP mux.

func (b *Bot) RegisterHandlers() {
	http.HandleFunc("/webhook", b.handleWebhook)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

func (b *Bot) handleWebhook(_ http.ResponseWriter, r *http.Request) {
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
	// 0. Handle Admin Commands
	if msg.Text == "/metrics" {
		if msg.From.ID != b.cfg.AdminTelegramID {
			b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⛔ *Access Denied*: Admin only."))
			return
		}
		b.handleMetricsCommand(msg.Chat.ID)
		return
	}

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

		plan, metas, err := b.planner.GeneratePlan(ctx, msg.Text, pCtx)

		// Record Metrics even if it errored (if we have metas)
		for _, m := range metas {
			_ = b.metricsStore.Record(metrics.ExecutionMetric{
				AgentName:        m.AgentName,
				Model:            m.Usage.Model,
				PromptTokens:     m.Usage.PromptTokens,
				CompletionTokens: m.Usage.CompletionTokens,
				LatencyMS:        m.Latency.Milliseconds(),
			})
			// Alert on Context Bloat
			if m.Usage.PromptTokens > 4000 {
				alert := fmt.Sprintf("⚠️ *Context Bloat Alert*\nAgent: Planner\nModel: %s\nPrompt Tokens: %d", m.Usage.Model, m.Usage.PromptTokens)
				b.sendAdminAlert(alert)
			}
		}

		if err != nil {
			log.Printf("Error generating plan: %v", err)
			safeErr := strings.ReplaceAll(err.Error(), "`", "'")
			finalText := fmt.Sprintf("❌ *Error generating plan:*\n```\n%v\n```", safeErr)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
		} else {
			// Save the generated meal plan to user memory
			userID := fmt.Sprintf("%d", msg.From.ID)
			if err := b.planRepo.Save(ctx, userID, plan); err != nil {
				log.Printf("Warning: failed to save meal plan to user memory for user %s: %v", userID, err)
			}

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

	recipeWithEmbedding, meta, err := recipe.NormalizeHTML(
		ctx,
		b.textGen,
		b.embedGen,
		recipe.PostData{
			ID:        post.ID,
			Title:     post.Title,
			UpdatedAt: post.UpdatedAt,
			HTML:      post.HTML,
		},
	)

	b.metricsStore.Record(metrics.ExecutionMetric{
		AgentName:        meta.AgentName,
		Model:            meta.Usage.Model,
		PromptTokens:     meta.Usage.PromptTokens,
		CompletionTokens: meta.Usage.CompletionTokens,
		LatencyMS:        meta.Latency.Milliseconds(),
	})

	if err != nil {
		log.Printf("Background Error: Failed to normalize '%s': %v", post.Title, err)
		return
	}

	// Save to new RecipeRepository
	if err := b.recipeRepo.Save(ctx, recipeWithEmbedding.Recipe); err != nil {
		log.Printf("Background Error: Failed to save recipe '%s' to DB: %v", recipeWithEmbedding.Title, err)
		return
	}
	// Save embedding to new VectorRepository
	if err := b.vectorRepo.Save(ctx, recipeWithEmbedding.ID, recipeWithEmbedding.Embedding); err != nil {
		log.Printf("Background Error: Failed to save embedding for '%s' to DB: %v", recipeWithEmbedding.Title, err)
		return
	}

	log.Printf("Background Success: Recipe '%s' is now indexed and searchable.", post.Title)
}

func (b *Bot) handleMetricsCommand(chatID int64) {
	usage, err := b.metricsStore.GetDailyUsage(7)
	if err != nil {
		b.api.Send(tgbotapi.NewMessage(chatID, "❌ Error fetching metrics."))
		return
	}

	health := metrics.GetSysHealth("data")

	var sb strings.Builder
	sb.WriteString("📊 *Usage & Health Report*\n\n")

	sb.WriteString("🗓 *Recent LLM Activity*\n")
	if len(usage) == 0 {
		sb.WriteString("_No data yet_\n")
	}
	for _, d := range usage {
		sb.WriteString(fmt.Sprintf("• *%s*: %d tokens (%d execs)\n", d.Date, d.TotalPrompt+d.TotalCompletion, d.TotalExecution))
	}

	sb.WriteString("\n🧠 *System Health*\n")
	sb.WriteString(fmt.Sprintf("• RAM: %dMB (Alloc) / %dMB (Sys)\n", health.AllocMB, health.SysMB))
	sb.WriteString(fmt.Sprintf("• Goroutines: %d\n", health.Goroutines))
	sb.WriteString(fmt.Sprintf("• Disk Data: %s\n", health.DataDiskSize))

	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}

func (b *Bot) sendAdminAlert(text string) {
	if b.cfg.AdminTelegramID == 0 {
		return
	}
	msg := tgbotapi.NewMessage(b.cfg.AdminTelegramID, text)
	msg.ParseMode = "Markdown"
	b.api.Send(msg)
}
