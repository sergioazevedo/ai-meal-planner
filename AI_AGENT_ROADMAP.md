# AI Agent Learning Roadmap: Beyond Deterministic Pipelines

This guide tracks the transition from building LLM-powered applications to engineering true **Agentic Workflows**. 

---

## 1. Where You Are Now: The Multi-Agent Pipeline
You have successfully moved past basic LLM calls and implemented a **Multi-Agent Architecture** with Human-in-the-Loop (HitL) capabilities.
*   **Structured Handover:** Passing strictly typed JSON between specialized nodes.
*   **Role Separation:** Distinguishing between Strategy (`Analyst`), Tactics (`Chef`), and Revision (`Reviewer`).
*   **Grounding (RAG):** Using embeddings to prevent hallucinations and strictly constrain the LLM to known recipes.
*   **Human-in-the-Loop:** Allowing the user (via Telegram) to critique the plan and having an agent (`Reviewer`) dynamically adjust it.

---

## 2. The Next Frontier: Dynamic Agency & Self-Correction

### Phase 1: Tool Use & Function Calling (The Autonomous Analyst)
**Concept:** Instead of the Go code *pushing* data to the LLM, the LLM *pulls* the data it needs.
*   **Current State:** The Go backend runs a semantic search, grabs 30 recipes, and hands them to the Analyst in a single prompt.
*   **The Upgrade:** Refactor the `Planner` so the `Analyst` is given a tool definition (e.g., `search_recipes(query: string, limit: int)`). 
*   **Why it matters:** If the user asks for "High-protein breakfasts and vegan dinners", the Analyst might realize the initial context block doesn't have enough vegan options. It can autonomously execute `search_recipes("vegan dinner")` to gather more context before writing the plan.

### Phase 2: Automated Self-Correction (The Internal Critic Loop)
**Concept:** Agents "talking back" to each other to fix errors *before* the user ever sees the output.
*   **The Workflow:** `Analyst` -> `Chef` -> `Validator (Internal QA)`. 
*   **The Loop:** If the `Validator` finds a flaw (e.g., "The user explicitly asked for no dairy, but you included a recipe with cheese"), it automatically sends the plan back to the `Analyst` with a strict error message to try again (up to 3 retries).
*   **Why it matters:** It guarantees a higher floor of quality. The Telegram user should only see plans that have already passed internal dietary and structural checks.

### Phase 3: Persistent Memory & Personalization
**Concept:** Moving beyond short-term session state to long-term user profiles.
*   **Current State:** The bot remembers the context of a conversation for a short time via the `UserSession` table, but forgets long-term preferences.
*   **The Upgrade:** Create an "Extraction Agent" (using a fast, cheap model like Llama 3 8B) that runs in the background of every Telegram chat. If a user says "I hate salmon" or "I am allergic to peanuts", the agent writes this to a `user_preferences` SQLite table.
*   **Why it matters:** These long-term preferences are permanently injected into the `PlanningContext` for that specific user, creating a deeply personalized AI Chef over time.

### Phase 4: LLM-as-a-Judge (Scientific Evals)
**Concept:** Using an advanced LLM to measure the quality of your workflow.
*   **Current State:** You have an evaluation framework (`analyst_eval_test.go`), but it relies on hardcoded string matching (e.g., checking if `recipe.Title == "Spicy Chili"`).
*   **The Upgrade:** Create a "Golden Set" of hard user requests. Run your agents, then use a higher-tier model (e.g., Gemini 1.5 Pro) to score the final meal plan against a rubric (e.g., "Score 1-10 on variety, constraint adherence, and macro-balance").
*   **Why it matters:** It turns prompt engineering from "vibes" into a measurable science, ensuring that tweaks to the `Chef`'s prompt don't accidentally ruin the output quality.

---

## 3. Key Distinction
*   **Pipeline (Where we are):** Code -> DB Search -> Agent (`Analyst`) -> Agent (`Chef`) -> User.
*   **Agentic (Where we are going):** Code -> Agent (`Analyst`) -> (Agent decides to use DB Search Tool) -> Agent (`Chef`) -> Agent (`Validator QA`) -> (Validator decides to Loop back to Analyst) -> User.
