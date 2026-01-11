# Project Instructions — AI-Assisted Recipe Meal Planner

## Overview
This project generates **weekly meal plans** from recipes stored in Ghost CMS.
It uses a **hybrid RAG pipeline**: semantic search + external LLM (Gemini) for meal plan generation.

---

## Architecture & Workflow

### Step 1: Fetch & Normalize Recipes
1. Fetch posts from **Ghost CMS** using the Content API.
2. Send each post’s **HTML content** to an LLM parser:
   - Goal: convert HTML into **structured JSON**:
     ```json
     {
       "title": "Recipe Name",
       "ingredients": ["ingredient 1", "ingredient 2", ...],
       "instructions": "Step-by-step instructions",
       "tags": ["tag1", "tag2"]
     }
     ```
   - The LLM ensures consistency in formatting, extracts optional metadata (diet, prep time, etc.).
3. Store the normalized JSON locally or in a database.

### Step 2: Generate Embeddings + Build RAG
1. Generate **embeddings** for each normalized recipe:
   - Options: local CPU-friendly embedding model or external API.
2. Store embeddings in a **vector database**:
   - SQLite + vector extension, Chroma, or pgvector.
3. When a user requests a meal plan:
   - Retrieve **top N relevant recipes** using semantic search.
   - Construct a prompt including the retrieved recipes and user preferences.
   - Send prompt to **Gemini LLM** for weekly meal plan generation.
4. Parse the JSON output and present it to the user.

---

## Functional Requirements
- Fetch recipes from Ghost CMS.
- Normalize recipes to JSON.
- Generate embeddings for semantic search.
- Generate weekly meal plans using Gemini.
- Include optional shopping list aggregation.
- Respect dietary preferences and repetition rules.

---

## Non-Functional Requirements
- Low hosting cost (Lightsail smallest instance).
- Simple, monolithic service (Go recommended).
- No Kubernetes or GPU required.
- Easy to update: new recipes automatically re-indexed.
- Maintainable: clear separation between ingestion, embeddings, and generation.

---

## Deployment
- Ghost runs on Lightsail as the recipe CMS.
- Go service runs on the same or separate Lightsail instance.
- Scheduled jobs:
  - Cron for recipe ingestion + normalization.
  - Optional cron for weekly meal plan generation.
- External LLM: Gemini (free tier recommended).

---

## Notes
- AGENT.MD contains the **LLM system prompt and generic agent instructions**.
- PROJECT.MD contains **project-specific implementation workflow**.
- Both files are version-controlled and updated independently.

## Implementation standard

- DO NOT over engineer things. Start with the simplest implementation.
- Always keep the performance and security as a first priority.
- Ask for any clarification rather just guessing things if you are not clear about anything.

### Testing Strategy

- **Unit Tests:**
  - Required for all business logic (packages in `internal/`).
  - Must use standard Go testing patterns (table-driven tests preferred).
  - Mock external dependencies (like the Gemini API) to ensure speed and reliability.

- **Acceptance Tests:**
  - Placed in `acceptance_tests/`.
  - Focus on end-to-end user workflows (e.g., "Generate a meal plan from scratch").
  - Should run against a local environment or staged inputs.
