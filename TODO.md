# Project Todo List

## Phase 1: Ingestion & Normalization (Current Status: Mostly Complete)
- [x] Fetch recipes from Ghost CMS (`internal/ghost`)
- [x] Normalize HTML to JSON using Gemini (`internal/recipe`)
- [x] Store normalized recipes locally (`internal/storage`)
- [x] Basic CLI entry point to run ingestion (`cmd/ai-meal-planner`)

## Phase 1.5: Reliable LLM Inference (High Priority)
- [x] **Switch to Hybrid LLM Stack (Groq + Gemini)**
  - [x] Implement Groq API client for text generation (Normalization & Planning).
  - [x] Use Llama 3 (70B) or Mixtral for reliable, high-speed inference.
  - [x] Keep Gemini strictly for `text-embedding-004` (Embeddings) to bypass text rate limits.
  - [x] Update `.env` and `config` to support `GROQ_API_KEY`.
- [x] **Enhance Normalization**
  - [x] Add `PrepTime` and `Servings` fields to `NormalizedRecipe` struct.
  - [x] Update LLM prompt to extract/estimate these values from the full HTML.
  - [x] Update unit and acceptance tests to verify these new fields.

## Phase 2: RAG Pipeline (Embeddings & Search)
- [x] **Implement Embeddings Generation**
  - Add functionality to generate vector embeddings for normalized recipes (using Gemini Embedding API or local model).
  - Update `NormalizedRecipe` or create a new structure to hold embeddings.
- [x] **Implement Vector Storage & Retrieval**
  - Choose and implement a vector store (e.g., SQLite with vector extension, or a simple in-memory store if scale permits, or `pgvector`).
  - Implement "Semantic Search" to find recipes matching user queries/preferences.

## Phase 3: Meal Planning Logic
- [x] **Implement Planner Logic** (`internal/planner`)
  - Create the logic to accept user constraints (e.g., "low carb", "vegetarian").
  - Retrieve relevant recipes using the vector search.
  - Construct the context-rich prompt for Gemini.
- [x] **Generate Meal Plan**
  - specific function to parse Gemini's response into a structured Weekly Meal Plan.

## Phase 4: Application Polish & CLI
- [x] **CLI Improvements**
  - Refactor `main.go`/`app.go` to support subcommands (e.g., `ingest` vs `plan`).
  - Add flags for user preferences (e.g., `--diet vegetarian`).
- [ ] **Shopping List (Optional)**
  - Aggregate ingredients from the generated plan. (Already partially implemented in the JSON response)

## Phase 5: Testing & Deployment
- [x] **Acceptance Tests**
  - Expand `acceptance_tests/` to cover the full "Generate Plan" flow.
- [x] **CI/CD & Deployment**
  - Setup build/release scripts. (Dockerfile created)
  - Create deployment documentation (`DEPLOY.md`).
- [ ] **Automation & Monitoring**
  - Setup hourly cron job for ingestion on VPS.
  - Add error alerts if ingestion fails.
  - Implement smarter skipping for unchanged recipes (validate timestamp logic).

## Phase 6: Interfaces (Next Steps)
- [x] **Telegram Bot Integration (Webhook-based)**
  - [x] Create a new `cmd/telegram-bot` entry point.
  - [x] Implement a lightweight web app (HTTP server) to handle incoming webhooks from Telegram.
  - [x] Add logic to set the Telegram webhook URL during application startup.
  - [x] Map user messages from webhooks to the `Planner.GeneratePlan` function.
  - [x] Render the output as formatted Markdown messages back to the user.
  - [x] **Security:** Whitelist only your specific Telegram User ID in the webhook handler.
  - [x] **Feature: Recipe Clipper / Importer**
    - [x] Accept a URL sent by the user.
    - [x] Fetch the HTML content of the URL.
    - [x] Use Groq to extract the recipe and format it as a Ghost Post (Title, HTML Body, Tags).
    - [x] **Upgrade:** Implement Ghost Admin API (Write Access) to create and publish a new post on the blog.

