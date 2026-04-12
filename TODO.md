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
- [ ] **Feature: Advanced Planning Options**
    - [x] Support multiple Telegram IDs for access control.
    - [x] Improve message formatting (split Menu and Shopping List).
    - [x] Support "Cooking Frequency" (e.g., cook 3 times for 7 days of food) by adjusting portions/leftovers.
    - [x] Support "Household Composition" (Number of adults, children, and ages) for precise ingredient scaling.
    - [x] Update LLM prompts to incorporate these household and frequency constraints.

## Phase 7: Multi-Agent Architecture (Evolution)
- [ ] **Refactor Planner into a Multi-Agent Pipeline**
    - [x] **Agent 1: The Analyst** - Responsible for recipe selection and batch-cooking strategy.
    - [ ] **Agent 2: The Nutritionist** - Basic version to audit balance, variety, and healthy heuristics.
    - [x] **Agent 3: The Chef** - Responsible for final scheduling and JSON synthesis.
    - [ ] **Agent 4: The Grocer** - Responsible for organizing and categorizing the shopping list by supermarket aisle.
- [x] **Implement Orchestrator Pattern**
    - [x] Create a "Manager" logic in `internal/planner` to coordinate hand-offs and state between agents (Using the Generic Agent Engine).
- [x] **Enhanced Testing**
    - [x] Add unit tests for each agent's specialized prompt and output (Live Evals).

## Phase 8: Advanced RAG & Search Optimization
- [ ] **Metadata Pre-filtering**
    - [ ] Update `NormalizedRecipe` to include strict dietary flags (e.g., `is_vegan`, `is_vegetarian`, `is_gluten_free`, `is_dairy_free`).
    - [ ] Update the Normalization prompt to accurately extract these flags.
    - [ ] Modify the search logic to apply SQL `WHERE` filters *before* calculating vector similarity to ensure dietary constraints are 100% respected.
- [x] **In-Database Vector Search**
    - [x] Implement Random Discovery using native SQL `ORDER BY RANDOM()`.
    - [ ] Migrate from the current in-memory Go similarity loop to a native SQLite vector extension (e.g., `sqlite-vec`).
- [ ] **Hybrid Search (Keyword + Vector)**
    - [ ] Enable SQLite FTS5 (Full Text Search) for recipe titles and ingredient lists.
    - [ ] Combine keyword matches with semantic vector results (using a technique like Reciprocal Rank Fusion) to handle specific ingredient requests more accurately.

## Phase 9: Pure Agentic RAG Migration (Current Status: Complete)
- [x] **Implement Tool-Enabled Analyst**
    - [x] Update `internal/llm/llm.go` to support Tool Definitions and Tool Calls (universal schema).
    - [x] Implement tool-calling logic in `internal/llm/gemini.go` and `internal/llm/groq.go`.
    - [x] Update `internal/planner/analyst.go` to implement the Agent Loop.
    - [x] Update `internal/planner/analyst_prompt.md` to include tool descriptions and strategic rules.
- [x] **Implement Critic/Reviewer Feedback Loop**
    - [x] Implement `PlanReviewer` agent to intelligently revise plans based on user feedback.
    - [x] Add multi-turn autonomous loops for plan adjustment.
    - [x] Implement mechanical guardrails (`maxTurns`) and error handling for tool hallucinations.
- [x] **The Deterministic Trap Fix**
    - [x] Split `search_recipes` into `semantic` and `random` tools.
    - [x] Remove pre-fetching and trust LLM agency.
- [ ] **Implement Nutritionist as Consultant**
    - [ ] Add `get_nutrition_advice` tool to the Analyst's toolset.
    - [ ] Implement a lightweight Nutritionist tool/agent to provide meal alternatives.