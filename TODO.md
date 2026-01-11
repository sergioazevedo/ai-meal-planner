# Project Todo List

## Phase 1: Ingestion & Normalization (Current Status: Mostly Complete)
- [x] Fetch recipes from Ghost CMS (`internal/ghost`)
- [x] Normalize HTML to JSON using Gemini (`internal/recipe`)
- [x] Store normalized recipes locally (`internal/storage`)
- [x] Basic CLI entry point to run ingestion (`cmd/ai-meal-planner`)
- [ ] **Enhance Normalization (High Priority)**
  - Add `PrepTime` and `Servings` fields to `NormalizedRecipe` struct.
  - Update LLM prompt to extract/estimate these values from the full HTML.
  - Update unit and acceptance tests to verify these new fields.

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
- [ ] **CLI Improvements**
  - Refactor `main.go`/`app.go` to support subcommands (e.g., `ingest` vs `plan`).
  - Add flags for user preferences (e.g., `--diet vegetarian`).
- [ ] **Shopping List (Optional)**
  - Aggregate ingredients from the generated plan.

## Phase 5: Testing & Deployment
- [ ] **Acceptance Tests**
  - Expand `acceptance_tests/` to cover the full "Generate Plan" flow.
- [ ] **CI/CD & Deployment**
  - Setup build/release scripts.
