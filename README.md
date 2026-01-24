# AI-Assisted Recipe Meal Planner

A smart CLI tool that connects to your **Ghost CMS** blog, learns your recipes using **Google Gemini**, and generates personalized weekly meal plans based on your requests.

## 🚀 Features

*   **Ghost CMS Integration**: Automatically fetches and updates recipes from your blog.
*   **AI Normalization**: Uses Gemini 1.5 Pro and Groq Llama3 7b to extract structured data (ingredients, steps, prep time, servings) from raw HTML posts.
*   **RAG Pipeline**: Generates vector embeddings for every recipe and performs local semantic search to find the best matches for your cravings.
*   **Smart Planning**: Creates a complete 7-day meal plan with a consolidated shopping list.
*   **Recipe Clipper**: Send any recipe URL to the Telegram bot; it extracts the details, publishes them to your Ghost blog, and immediately indexes them for planning.
*   **Batch Cooking & Leftovers**: Smart scheduling that understands you might only want to cook 3-4 times a week, automatically filling gaps with leftovers.
*   **Household Scaling**: Automatically adjusts ingredient quantities based on your household composition (Adults vs. Children).
*   **Multi-User Support**: Whitelist multiple Telegram IDs for shared household planning.
*   **Telegram Bot Interface**: Chat with your planner, request meals, and get instant markdown plans on your phone.
*   **Zero-Database**: Uses a highly efficient flat-file storage system with versioned caching.

## 🛠️ Prerequisites

You need the following API keys:
1.  **Ghost Content & Admin API Keys**: To fetch and publish recipes.
2.  **Google Gemini API Key**: For embeddings (free tier available).
3.  **Groq API Key**: For high-speed LLM inference (free tier available).

## ⚙️ Configuration

Set these variables in your `.env` file or environment:

| Variable | Description | Default |
| :--- | :--- | :--- |
| `GHOST_URL` | Your Ghost blog base URL | Required |
| `GHOST_CONTENT_API_KEY` | Ghost Content API Key | Required |
| `GHOST_ADMIN_API_KEY` | Ghost Admin API Key (for Clipper) | Required |
| `GEMINI_API_KEY` | Google Gemini API Key | Required |
| `GROQ_API_KEY` | Groq API Key | Required |
| `TELEGRAM_BOT_TOKEN` | Token from @BotFather | Optional (Bot) |
| `TELEGRAM_ALLOWED_USER_IDS`| Comma-separated list of allowed IDs | Optional (Bot) |
| `DEFAULT_ADULTS` | Number of adults for scaling | `2` |
| `DEFAULT_CHILDREN` | Number of children for scaling | `1` |
| `DEFAULT_CHILDREN_AGES` | Comma-separated ages (e.g., `5,8`) | `5` |
| `DEFAULT_COOKING_FREQUENCY`| How many times per week to cook | `4` |
| `RECIPE_STORAGE_PATH` | Where to store normalized JSONs | `data/recipes` |

## ⚡ Quick Start

### 1. Clone the repository
```bash
git clone https://github.com/your-user/ai-meal-planner.git
cd ai-meal-planner
```

### 2. Configure Environment
Set the required environment variables. You can export them in your shell or use a `.env` file manager.

```bash
export GHOST_URL="https://your-blog.com"
export GHOST_API_KEY="your_ghost_content_key"
export GEMINI_API_KEY="your_google_gemini_key"
export GROQ_API_KEY="your_groq_api_key"
```

### 3. Run the CLI

**Step 1: Ingest Recipes**
Fetch recipes from Ghost and build the local vector index. Run this whenever you add new posts.
```bash
go run ./cmd/ai-meal-planner ingest
```

**Step 2: Generate a Plan**
Ask for a plan using natural language.
```bash
go run ./cmd/ai-meal-planner plan -request "I want healthy vegetarian dinners, quick to make"
```

## 🤖 Telegram Bot (Optional)

You can interact with your meal planner directly via Telegram.

### 1. Create a Bot
Message [@BotFather](https://t.me/BotFather) on Telegram to create a new bot and get your **API Token**.

### 2. Local Development with Localtunnel
Since Telegram uses webhooks, your local server must be accessible from the internet. We recommend `localtunnel` for quick setup (no signup required):

```bash
# 1. Start your local tunnel in a separate terminal
npx localtunnel --port 8080

# 2. Copy the generated URL (e.g., https://fresh-apple-move.loca.lt)

# 3. Export the required variables
export TELEGRAM_BOT_TOKEN="your_bot_token"
export TELEGRAM_ALLOW_USER_ID="your_numeric_id"
export TELEGRAM_WEBHOOK_URL="https://fresh-apple-move.loca.lt/webhook"

# 4. Run the bot
go run ./cmd/telegram-bot
```

## 📦 Deployment

This application compiles to a single static binary, making it perfect for low-cost servers like **AWS Lightsail**.

*   **Direct Binary**: Copy the file and run (Recommended).
*   **Docker**: Run as a container.
*   **Automation**: Use `cron` to keep recipes in sync.

👉 **[Read the Full Deployment Guide](DEPLOY.md)**

## 🏗️ Architecture

1.  **Ingestion Service**: Pulls content from Ghost -> Normalizes via LLM -> Saves JSON + Embeddings.
2.  **Storage**: Local JSON files act as both the database and the vector index.
3.  **Planner Service**: Embeds user query -> Finds nearest recipe neighbors (Cosine Similarity) -> Generates Plan via LLM.

## 🔮 Roadmap

*   [x] Core CLI & RAG Pipeline
*   [x] Telegram Bot Integration
*   [x] Recipe Clipper / Importer
*   [x] Batch Cooking & Household Scaling
*   [x] Multi-user support
*   [ ] Shopping List Export (PDF/Email)
*   [ ] User Accounts (Web Interface)

## 📄 License
MIT
