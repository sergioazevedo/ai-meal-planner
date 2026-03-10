# AI Testing Strategy: From Assertions to Evals

As the Meal Planner transitions from a deterministic pipeline to an Agentic Workflow, our testing strategy must evolve. Traditional unit tests (checking if `string A == string B`) are insufficient for testing the non-deterministic output of Large Language Models.

This document outlines our strategy for ensuring the quality, safety, and constraint-adherence of our AI agents.

---

## 1. The Testing Pyramid

Our testing suite is divided into three tiers:

1.  **Standard Unit Tests (Fast):**
    *   **What:** Testing basic Go logic, JSON unmarshaling, database queries, and URL parsing.
    *   **Tools:** Standard `go test`.
    *   **Execution:** Runs on every local save and every GitHub push.

2.  **Acceptance Tests (Medium):**
    *   **What:** Testing the *plumbing* of the application (e.g., "Can we fetch from Ghost, save to SQLite, and generate a dummy plan without crashing?").
    *   **Tools:** `database.NewDB` with migrations, mock LLM clients.
    *   **Execution:** Runs on every GitHub push.

3.  **LLM Evals (Slow / Costly):**
    *   **What:** Testing the *reasoning* and *quality* of the AI agents using real LLM API calls.
    *   **Tools:** Groq/Gemini APIs, Golden Dataset, LLM-as-a-Judge.
    *   **Execution:** Runs via GitHub Actions on PRs to `main` (using `-run LiveEval`).

---

## 2. The "Golden Set" Seed Data

To evaluate an LLM's reasoning, the environment must be tightly controlled. We cannot run evaluations against the live Ghost CMS database, as adding or removing a recipe would break the test's baseline.

**The Solution:**
We will maintain a static, curated dataset in `testdata/golden_recipes.json`.

**How to Create the Golden Set from Production:**
The most accurate way to build this dataset is by extracting real recipes from the production database. You can export a raw sample using the `sqlite3` CLI tool on your production server:

```bash
sqlite3 your_production_database.db "SELECT data FROM recipes LIMIT 50;" > raw_recipes_export.txt
```
Once exported, this raw JSON text can be reviewed, hand-picked for the edge cases below, and formatted into `testdata/golden_recipes.json`.

**Requirements for the Golden Set:**
*   **Size:** ~20-30 highly diverse recipes.
*   **Edge Cases:**
    *   *The "Fake-Out":* A recipe tagged "Vegan" that lists "Honey" in the ingredients (to test if the Nutritionist catches the error).
    *   *The Extremes:* 10-minute instant meals vs. 8-hour slow cooker meals.
    *   *Ingredient Overlap:* Recipes that clearly share perishable ingredients (e.g., half a cabbage, cilantro) to test the Analyst's batch-cooking strategy.
    *   *Strict Dietary:* Distinct buckets of Keto, Paleo, Gluten-Free, and Nut-Free.

**Execution Flow in Evals:**
1. Spin up an in-memory SQLite database (`:memory:` or temp file).
2. Run schema migrations (`db.MigrateUp`).
3. Ingest the `golden_recipes.json` into the `recipes` table.
4. Generate and store embeddings for this exact set.
5. Run the Planner against this controlled context.

---

## 3. LLM-as-a-Judge (The Evaluation Mechanism)

We will replace hardcoded Go assertions (`if plan.Contains("Chicken")`) with a secondary, high-reasoning LLM (e.g., Gemini 1.5 Pro) that acts as the evaluator.

### The Mechanism:
1.  **The Subject:** The Groq `Analyst` and `Chef` generate a plan based on a difficult prompt (e.g., "3 days of high-protein vegan, under 30 mins").
2.  **The Prompt:** We pass the user's original request, the generated meal plan JSON, and a strict Rubric to the Judge model.
3.  **The Rubric Output:** The Judge returns a structured JSON evaluation:

```json
{
  "constraint_adherence_score": 10,
  "variety_score": 8,
  "strategy_score": 9,
  "failures": [],
  "reasoning": "The plan perfectly adhered to the vegan and time constraints. Variety was good, though two meals used a tomato base."
}
```

4.  **The Assertion:** The Go test asserts that the `constraint_adherence_score` is exactly 10, and the average score is >= 8.5. If not, the test fails (`t.Fatalf`), and the PR is blocked.

---

## 4. Immediate Action Items

- [ ] Create `testdata/golden_recipes.json` with the first 10 edge-case recipes.
- [ ] Refactor `analyst_eval_test.go` to load from the Golden Set instead of using inline Go structs.
- [ ] Create a new `evaluator` package or agent that specifically handles the "LLM-as-a-Judge" prompting and parsing.
