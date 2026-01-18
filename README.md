# AI-Assisted Recipe Meal Planner

A smart CLI tool that connects to your **Ghost CMS** blog, learns your recipes using **Google Gemini**, and generates personalized weekly meal plans based on your requests.

## 🚀 Features

*   **Ghost CMS Integration**: Automatically fetches and updates recipes from your blog.
*   **AI Normalization**: Uses Gemini 1.5 Pro and Groq Llama3 70b to extract structured data (ingredients, steps, prep time, servings) from raw HTML posts.
*   **RAG Pipeline**: Generates vector embeddings for every recipe and performs local semantic search to find the best matches for your cravings.
*   **Smart Planning**: Creates a complete 7-day meal plan with a consolidated shopping list.
*   **Zero-Database**: Uses a highly efficient flat-file storage system with versioned caching.

## 🛠️ Prerequisites

You need the following API keys:
1.  **Ghost Content API Key & URL**: To fetch your recipes.
2.  **Google Gemini API Key**: For LLM processing and embeddings (free tier available).
3.  **Groq API Key**: For fast LLM responses (free tier available).

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
*   [ ] Shopping List Export (PDF/Email)
*   [ ] WhatsApp Interface (Chat with your planner)
*   [ ] User Accounts (Multi-user support)

## 📄 License
MIT
