package telegram

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"ai-meal-planner/internal/app"
	"ai-meal-planner/internal/clipper"
	"ai-meal-planner/internal/config"
	"ai-meal-planner/internal/ghost"
	"ai-meal-planner/internal/llm"
	"ai-meal-planner/internal/metrics"
	"ai-meal-planner/internal/planner"
	"ai-meal-planner/internal/recipe"
	"ai-meal-planner/internal/shopping"

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
	planRepo     *planner.PlanRepository
	recipeRepo   *recipe.Repository
	vectorRepo   *llm.VectorRepository
	shoppingRepo *shopping.Repository
	sessionRepo  *SessionRepository
	extractor    *recipe.Extractor // Added extractor
}

// NewBot initializes the Telegram Bot and sets the Webhook.
func NewBot(
	cfg *config.Config,
	planner *planner.Planner,
	clipper *clipper.Clipper,
	metricsStore *metrics.Store,
	textGen llm.TextGenerator,
	embedGen llm.EmbeddingGenerator,
	planRepo *planner.PlanRepository,     // New parameter
	recipeRepo *recipe.Repository,        // New parameter
	vectorRepo *llm.VectorRepository,     // New parameter
	shoppingRepo *shopping.Repository,    // New parameter
	sessionRepo *SessionRepository,       // New parameter
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

	extractor := recipe.NewExtractor(textGen, embedGen, vectorRepo)

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
		shoppingRepo: shoppingRepo,
		sessionRepo:  sessionRepo,
		extractor:    extractor,
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

func (b *Bot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	update, err := b.api.HandleUpdate(r)
	if err != nil {
		log.Printf("Error parsing update: %v", err)
		return
	}

	if update.CallbackQuery != nil {
		b.handleCallbackQuery(update.CallbackQuery)
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
	ctx := context.Background()
	userID := fmt.Sprintf("%d", msg.From.ID)

	// 0. Check for active session (e.g., awaiting adjustment feedback)
	session, err := b.sessionRepo.GetActive(ctx, userID, time.Now())
	if err != nil {
		log.Printf("Error checking session: %v", err)
	}
	if session != nil && session.SessionType == "adjust_plan" && session.State == "awaiting_feedback" {
		b.handleAdjustmentFeedback(ctx, msg, session)
		return
	}

	// 1. Handle Admin Commands
	if msg.Text == "/metrics" {
		b.handleMetricsRequest(msg)
		return
	}

	// 2. Detect if it's a URL (Clipper mode) or a request (Planner mode)
	if strings.HasPrefix(msg.Text, "http://") || strings.HasPrefix(msg.Text, "https://") {
		b.handleClipperRequest(msg)
		return
	}

	// 3. Default to Planner mode
	b.handlePlannerRequest(msg)
}

func (b *Bot) handleMetricsRequest(msg *tgbotapi.Message) {
	if msg.From.ID != b.cfg.AdminTelegramID {
		b.api.Send(tgbotapi.NewMessage(msg.Chat.ID, "⛔ *Access Denied*: Admin only."))
		return
	}
	b.handleMetricsCommand(msg.Chat.ID)
}

func (b *Bot) handleClipperRequest(msg *tgbotapi.Message) {
	statusText := "✂️ *Clipping recipe...* \n(Extracting and saving to your blog)"
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, statusText)
	replyMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(replyMsg)
	if err != nil {
		log.Printf("Failed to send initial reply: %v", err)
		return
	}

	ctx := context.Background()

	// Parse URL and optional tags
	// Format: http://url tag: t1, t2
	parts := strings.Split(msg.Text, " ")
	url := parts[0]
	var manualTags []string

	for i, p := range parts {
		if strings.ToLower(p) == "tag:" && i+1 < len(parts) {
			tagStr := strings.Join(parts[i+1:], " ")
			// Split by comma or space
			rawTags := strings.FieldsFunc(tagStr, func(r rune) bool {
				return r == ',' || r == ' '
			})
			for _, t := range rawTags {
				trimmed := strings.TrimSpace(t)
				if trimmed != "" {
					manualTags = append(manualTags, trimmed)
				}
			}
			break
		}
	}

	// --- Clipper Flow ---
	post, err := b.clipper.ClipURL(ctx, url, manualTags)
	var finalText string
	if err != nil {
		log.Printf("Error clipping recipe: %v", err)
		safeErr := strings.ReplaceAll(err.Error(), "`", "'")
		finalText = fmt.Sprintf("❌ *Error clipping recipe:*\n```\n%v\n```", safeErr)
	} else {
		tagDisplay := ""
		if len(post.Tags) > 0 {
			var tagNames []string
			for _, t := range post.Tags {
				tagNames = append(tagNames, t.Name)
			}
			tagDisplay = "\n*Tags:* " + strings.Join(tagNames, ", ")
		}
		finalText = fmt.Sprintf("✅ *Recipe Saved!*\n\n*Title:* %s\n*URL:* %s/%s%s", post.Title, b.cfg.GhostURL, post.ID, tagDisplay)
		// Trigger background ingestion so it becomes searchable for future plans
		go b.ingestClippedPost(*post)
	}
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
	edit.ParseMode = "Markdown"
	b.api.Send(edit)
}

func (b *Bot) handlePlannerRequest(msg *tgbotapi.Message) {
	statusText := "🧑‍🍳 *Thinking...* \n(Analyzing recipes and generating your plan)"
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, statusText)
	replyMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(replyMsg)
	if err != nil {
		log.Printf("Failed to send initial reply: %v", err)
		return
	}

	ctx := context.Background()

	// --- Planner Flow ---
	log.Printf("Generating plan for request: %s", msg.Text)

	userID := fmt.Sprintf("%d", msg.From.ID)
	nextMonday := planner.GetNextMonday(time.Now())

	// Check if plan already exists for next week
	exists, _ := b.planRepo.ExistsForWeek(ctx, userID, nextMonday)
	if exists {
		// Ask user what to do
		promptText := fmt.Sprintf("🗓️ A plan already exists for next week (starting *%s*).\nWhat would you like to do?", nextMonday.Format("2006-01-02"))

		// We need to keep the user request. Callback data is limited to 64 bytes.
		shortReq := msg.Text
		if len(shortReq) > 32 {
			shortReq = shortReq[:32]
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🔄 Redo Next Week", "redo|"+shortReq),
				tgbotapi.NewInlineKeyboardButtonData("⏭️ Plan Following Week", "next|"+shortReq),
			),
		)

		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, promptText)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		b.api.Send(edit)
		return
	}

	b.generateAndSendPlan(ctx, userID, msg.Chat.ID, sentMsg.MessageID, msg.Text, nextMonday)
}

func (b *Bot) handleCallbackQuery(query *tgbotapi.CallbackQuery) {
	ctx := context.Background()
	userID := fmt.Sprintf("%d", query.From.ID)
	data := query.Data

	parts := strings.Split(data, "|")
	if len(parts) < 2 {
		return
	}

	action := parts[0]

	// Answer callback to remove spinner
	b.api.Request(tgbotapi.NewCallback(query.ID, ""))

	switch action {
	case "confirm":
		b.handleConfirmDraft(ctx, query, userID, parts)
	case "adjust":
		b.handleAdjustDraft(ctx, query, userID, parts)
	case "startover":
		b.handleStartOver(ctx, query, userID, parts)
	case "redo", "next":
		// Legacy handlers for existing week conflict resolution
		request := parts[1]
		var targetWeek time.Time
		if action == "redo" {
			targetWeek = planner.GetNextMonday(time.Now())
		} else {
			targetWeek = planner.GetNextMonday(planner.GetNextMonday(time.Now()))
		}
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "🧑‍🍳 *Thinking...*")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		b.generateAndSendPlan(ctx, userID, query.Message.Chat.ID, query.Message.MessageID, request, targetWeek)
	}
}

func (b *Bot) generateAndSendPlan(ctx context.Context, userID string, chatID int64, messageID int, request string, targetWeek time.Time) {
	// Simple heuristic to extract context from natural language
	pCtx := planner.PlanningContext{
		Adults:           b.cfg.DefaultAdults,
		Children:         b.cfg.DefaultChildren,
		ChildrenAges:     b.cfg.DefaultChildrenAges,
		CookingFrequency: b.cfg.DefaultCookingFrequency,
	}

	plan, metas, err := b.planner.GeneratePlan(ctx, userID, request, pCtx, targetWeek)

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
		edit := tgbotapi.NewEditMessageText(chatID, messageID, finalText)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
	} else {
		// Set plan as DRAFT and clear shopping list (will be generated on confirm)
		plan.Status = planner.StatusDraft
		shoppingList := plan.ShoppingList // Save for later
		plan.ShoppingList = nil           // Clear from draft

		// Save the draft plan to database
		planID, err := b.planRepo.Save(ctx, userID, plan)
		if err != nil {
			log.Printf("Warning: failed to save meal plan to user memory for user %s: %v", userID, err)
		}
		plan.ID = planID

		// Store original request and shopping list in callback data
		// Format: confirm|planID|request (truncated to fit 64 byte limit)
		shortReq := request
		if len(shortReq) > 20 {
			shortReq = shortReq[:20]
		}

		callbackData := fmt.Sprintf("%d|%s", planID, shortReq)

		// Restore shopping list for potential later use
		plan.ShoppingList = shoppingList

		// Format plan text in concise format
		planText := formatDraftPlanMarkdown(plan)

		// Add feedback buttons
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm|"+callbackData),
				tgbotapi.NewInlineKeyboardButtonData("✏️ Adjust", "adjust|"+callbackData),
				tgbotapi.NewInlineKeyboardButtonData("🔄 Start Over", "startover|"+callbackData),
			),
		)

		// Edit message with the draft plan and buttons
		edit := tgbotapi.NewEditMessageText(chatID, messageID, planText)
		edit.ParseMode = "Markdown"
		edit.ReplyMarkup = &keyboard
		b.api.Send(edit)
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

	if err := app.ProcessAndSaveRecipe(
		ctx,
		b.extractor,
		b.recipeRepo,
		b.metricsStore,
		post,
	); err != nil {
		log.Printf("Background Error: Failed to process and save clipped post '%s': %v", post.Title, err)
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

// formatDraftPlanMarkdown formats a draft plan in concise format
func formatDraftPlanMarkdown(plan *planner.MealPlan) string {
	var sb strings.Builder
	sb.WriteString("📋 *DRAFT Meal Plan*\n\n")

	for _, dp := range plan.Plan {
		// Format: [Weekday1]-[Weekday2] - [Recipe Title] - [Prep time]
		sb.WriteString(fmt.Sprintf("*%s*: %s", dp.Day, dp.RecipeTitle))
		if dp.PrepTime != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", dp.PrepTime))
		}
		sb.WriteString("\n")
		if dp.Note != "" {
			sb.WriteString(fmt.Sprintf("_%s_\n", dp.Note))
		}
	}

	sb.WriteString("\n_Review your plan and choose an action below:_")
	return sb.String()
}

// handleConfirmDraft finalizes the draft plan and generates the shopping list
func (b *Bot) handleConfirmDraft(ctx context.Context, query *tgbotapi.CallbackQuery, userID string, parts []string) {
	if len(parts) < 2 {
		return
	}

	// Parse planID from callback data
	var planID int64
	fmt.Sscanf(parts[1], "%d|", &planID)

	// Get the draft plan from database
	plan, err := b.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		log.Printf("Error retrieving plan: %v", err)
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ *Error:* Could not retrieve plan.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		return
	}

	// Generate shopping list for the confirmed plan
	pCtx := planner.PlanningContext{
		Adults:           b.cfg.DefaultAdults,
		Children:         b.cfg.DefaultChildren,
		ChildrenAges:     b.cfg.DefaultChildrenAges,
		CookingFrequency: b.cfg.DefaultCookingFrequency,
	}

	shoppingListItems, err := b.planner.GenerateShoppingList(ctx, plan, pCtx)
	if err != nil {
		log.Printf("Error generating shopping list: %v", err)
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ *Error:* Could not generate shopping list.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		return
	}

	// Update plan's shopping list
	plan.ShoppingList = shoppingListItems

	// Update status to FINAL
	if err := b.planRepo.UpdateStatus(ctx, planID, planner.StatusFinal); err != nil {
		log.Printf("Error updating plan status: %v", err)
	}

	// Save shopping list
	if len(shoppingListItems) > 0 {
		shoppingList := &shopping.ShoppingList{
			UserID:     userID,
			MealPlanID: planID,
			Items:      shoppingListItems,
		}
		if _, err := b.shoppingRepo.Save(ctx, shoppingList); err != nil {
			log.Printf("Warning: failed to save shopping list: %v", err)
		}
	}

	// Format and send finalized plan
	planText, shoppingListText := formatPlanMarkdownParts(plan)

	// Edit message to show finalized plan (remove buttons)
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "✅ *Plan Confirmed!*\n\n"+planText)
	edit.ParseMode = "Markdown"
	b.api.Send(edit)

	// Send shopping list as second message
	shoppingMsg := tgbotapi.NewMessage(query.Message.Chat.ID, shoppingListText)
	shoppingMsg.ParseMode = "Markdown"
	b.api.Send(shoppingMsg)
}

// handleAdjustDraft initiates the adjustment workflow by creating a session and prompting for feedback
func (b *Bot) handleAdjustDraft(ctx context.Context, query *tgbotapi.CallbackQuery, userID string, parts []string) {
	// Parse planID from callback data
	var planID int64
	fmt.Sscanf(parts[1], "%d|", &planID)

	// Get the plan to extract original request
	plan, err := b.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		log.Printf("Error retrieving plan for adjustment: %v", err)
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ *Error:* Could not retrieve plan.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		return
	}

	// Create a session to track that we're awaiting adjustment feedback
	sessionCtx := SessionContextData{
		PlanID:          planID,
		OriginalRequest: "", // Will be populated from plan metadata if needed
	}

	sessionID, err := b.sessionRepo.Create(
		ctx,
		userID,
		"adjust_plan",
		"awaiting_feedback",
		sessionCtx,
		900, // 15 minute TTL for feedback
	)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ *Error:* Could not start adjustment mode.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		return
	}

	log.Printf("Created adjustment session %d for user %s, plan %d", sessionID, userID, planID)

	// Send a message asking for adjustment feedback
	feedbackPrompt := `✏️ *Plan Adjustment Mode*

Please describe what you'd like to change about your plan. Be specific about:

• *Which days* (e.g., "Monday", "Tuesday and Wednesday", "midweek")
• *What changes* (e.g., "make it vegetarian", "something faster", "no pasta", "use seasonal ingredients")

*Examples:*
- "Make Monday and Tuesday vegetarian"
- "Something faster for midweek"
- "No pasta recipes"
- "Use more seasonal ingredients"

Type your feedback below (or reply to this message):`

	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, feedbackPrompt)
	edit.ParseMode = "Markdown"
	b.api.Send(edit)
}

// handleStartOver deletes the draft and allows user to start fresh
func (b *Bot) handleStartOver(ctx context.Context, query *tgbotapi.CallbackQuery, userID string, parts []string) {
	// Parse planID and request from callback data
	var planID int64
	var request string
	if len(parts) >= 2 {
		dataParts := strings.SplitN(parts[1], "|", 2)
		fmt.Sscanf(dataParts[0], "%d", &planID)
		if len(dataParts) > 1 {
			request = dataParts[1]
		}
	}

	// Get the plan to find its week
	plan, err := b.planRepo.GetByID(ctx, planID)
	if err != nil || plan == nil {
		log.Printf("Error retrieving plan for start over: %v", err)
		edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "❌ *Error:* Could not retrieve plan.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		return
	}

	targetWeek := plan.WeekStart

	// Edit message to show "Thinking..."
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, "🔄 *Starting over...*\n🧑‍🍳 *Thinking...*")
	edit.ParseMode = "Markdown"
	b.api.Send(edit)

	// Use the original request to generate a new plan
	b.generateAndSendPlan(ctx, userID, query.Message.Chat.ID, query.Message.MessageID, request, targetWeek)
}

// handleAdjustmentFeedback processes user feedback to revise a meal plan
func (b *Bot) handleAdjustmentFeedback(ctx context.Context, msg *tgbotapi.Message, session *Session) {
	userID := fmt.Sprintf("%d", msg.From.ID)
	adjustmentFeedback := msg.Text

	// Show "thinking" message
	statusText := "✏️ *Revising plan...* \n(Analyzing your feedback)"
	replyMsg := tgbotapi.NewMessage(msg.Chat.ID, statusText)
	replyMsg.ParseMode = "Markdown"
	sentMsg, err := b.api.Send(replyMsg)
	if err != nil {
		log.Printf("Failed to send initial reply: %v", err)
		return
	}

	// Parse session context to get planID
	contextData, err := session.GetContextData()
	if err != nil {
		log.Printf("Error parsing session context: %v", err)
		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Invalid session data.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		b.sessionRepo.Delete(ctx, session.ID) // Clean up
		return
	}

	planID := contextData.PlanID

	// Retrieve the current plan
	currentPlan, err := b.planRepo.GetByID(ctx, planID)
	if err != nil || currentPlan == nil {
		log.Printf("Error retrieving plan for adjustment: %v", err)
		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not retrieve plan.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		b.sessionRepo.Delete(ctx, session.ID) // Clean up
		return
	}

	// Extract user request from the original plan data (if available)
	// For now, we'll use a generic request since we don't store it in the plan
	userRequest := "meal plan"

	// Perform RAG search for recipe candidates
	count, err := b.recipeRepo.Count(ctx)
	if err != nil {
		log.Printf("Error counting recipes: %v", err)
		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not search recipes.")
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		b.sessionRepo.Delete(ctx, session.ID) // Clean up
		return
	}

	var recipes []recipe.Recipe
	if count <= 20 {
		// For small pools, fetch all recipes
		recipes, err = b.recipeRepo.List(ctx, nil)
		if err != nil {
			log.Printf("Error listing recipes: %v", err)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not search recipes.")
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
			b.sessionRepo.Delete(ctx, session.ID) // Clean up
			return
		}
	} else {
		// For larger pools, use semantic search based on feedback
		queryEmbedding, err := b.embedGen.GenerateEmbedding(ctx, adjustmentFeedback)
		if err != nil {
			log.Printf("Error generating embedding: %v", err)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not search recipes.")
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
			b.sessionRepo.Delete(ctx, session.ID) // Clean up
			return
		}

		recipeIDs, err := b.vectorRepo.FindSimilar(ctx, queryEmbedding, 40, nil)
		if err != nil {
			log.Printf("Error searching similar recipes: %v", err)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not search recipes.")
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
			b.sessionRepo.Delete(ctx, session.ID) // Clean up
			return
		}

		recipes, err = b.recipeRepo.GetByIds(ctx, recipeIDs)
		if err != nil {
			log.Printf("Error fetching recipes: %v", err)
			edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, "❌ *Error:* Could not search recipes.")
			edit.ParseMode = "Markdown"
			b.api.Send(edit)
			b.sessionRepo.Delete(ctx, session.ID) // Clean up
			return
		}
	}

	log.Printf("PlanReviewer will choose from %d available recipes", len(recipes))

	// Call PlanReviewer agent
	pCtx := planner.PlanningContext{
		Adults:           b.cfg.DefaultAdults,
		Children:         b.cfg.DefaultChildren,
		ChildrenAges:     b.cfg.DefaultChildrenAges,
		CookingFrequency: b.cfg.DefaultCookingFrequency,
	}

	reviewerResult, err := b.planner.RunPlanReviewer(ctx, currentPlan, userRequest, adjustmentFeedback, pCtx, recipes)
	if err != nil {
		log.Printf("Error revising plan: %v", err)
		safeErr := strings.ReplaceAll(err.Error(), "`", "'")
		finalText := fmt.Sprintf("❌ *Error revising plan:*\n```\n%v\n```", safeErr)
		edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, finalText)
		edit.ParseMode = "Markdown"
		b.api.Send(edit)
		b.sessionRepo.Delete(ctx, session.ID) // Clean up
		return
	}

	// Record metrics
	_ = b.metricsStore.Record(metrics.ExecutionMetric{
		AgentName:        reviewerResult.Meta.AgentName,
		Model:            reviewerResult.Meta.Usage.Model,
		PromptTokens:     reviewerResult.Meta.Usage.PromptTokens,
		CompletionTokens: reviewerResult.Meta.Usage.CompletionTokens,
		LatencyMS:        reviewerResult.Meta.Latency.Milliseconds(),
	})

	// Check for context bloat
	if reviewerResult.Meta.Usage.PromptTokens > 4000 {
		alert := fmt.Sprintf("⚠️ *Context Bloat Alert*\nAgent: PlanReviewer\nModel: %s\nPrompt Tokens: %d", reviewerResult.Meta.Usage.Model, reviewerResult.Meta.Usage.PromptTokens)
		b.sendAdminAlert(alert)
	}

	// Update the plan with revised version (keep as DRAFT)
	revisedPlan := reviewerResult.RevisedPlan
	revisedPlan.Status = planner.StatusDraft
	shoppingList := revisedPlan.ShoppingList
	revisedPlan.ShoppingList = nil // Clear from draft

	// Save revised plan (creates a new plan record)
	newPlanID, err := b.planRepo.Save(ctx, userID, revisedPlan)
	if err != nil {
		log.Printf("Warning: failed to save revised meal plan: %v", err)
	}
	revisedPlan.ID = newPlanID
	revisedPlan.ShoppingList = shoppingList

	// Format the revised plan
	shortReq := userRequest
	if len(shortReq) > 20 {
		shortReq = shortReq[:20]
	}
	callbackData := fmt.Sprintf("%d|%s", newPlanID, shortReq)

	planText := formatDraftPlanMarkdown(revisedPlan)

	// Add feedback buttons
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", "confirm|"+callbackData),
			tgbotapi.NewInlineKeyboardButtonData("✏️ Adjust", "adjust|"+callbackData),
			tgbotapi.NewInlineKeyboardButtonData("🔄 Start Over", "startover|"+callbackData),
		),
	)

	// Edit message with the revised draft plan
	edit := tgbotapi.NewEditMessageText(msg.Chat.ID, sentMsg.MessageID, planText)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &keyboard
	b.api.Send(edit)

	// Delete the session (adjustment feedback processed)
	_ = b.sessionRepo.Delete(ctx, session.ID)
}
