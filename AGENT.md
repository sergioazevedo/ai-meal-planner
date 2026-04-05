# Project Agents

This document describes the conceptual AI agents used in the Meal Planner project. An agent is a logical component that uses a Large Language Model (LLM) to perform a specific reasoning task.

---

## CLI Agent Operational Rules
- **Code Execution:** Never start writing code or executing file modification commands without explicit confirmation or instruction from the user. Always propose the plan first and wait for approval.
- **Code Architecture Review:** Before introducing any new types, interfaces, or abstractions to solve a problem, propose the idea to the user for review.

---

## 1. Normalization Agent

### Purpose
To convert unstructured recipe HTML content from the Ghost CMS into a consistent, structured JSON format. This ensures that the data is clean and reliable for all downstream tasks, like meal planning and embedding.

### Input
A single string containing the raw HTML of a recipe post.

### Output
A single, clean JSON object matching the following schema. The agent must not produce any other text or formatting outside of the JSON object.

```json
{
  "title": "Recipe Name",
  "ingredients": ["ingredient 1", "ingredient 2", ...],
  "instructions": "Step-by-step instructions",
  "tags": ["tag1", "tag2"],
  "prep_time": "30 mins",
  "servings": "4 people"
}
```

---

## 2. Meal Plan Agent (Analyst)

### Purpose
To strategically generate a weekly meal plan based on user preferences. Unlike basic agents, the Analyst uses a **"Pull" model**, meaning it does not receive all recipes upfront. It autonomously searches for recipes as needed.

### Interaction Logic
1.  **Reasoning**: The agent analyzes the user request and identifies missing information or recipe needs.
2.  **Tool Use**: It calls the `search_recipes` tool with specific queries to fetch relevant candidates from the RAG pipeline.
3.  **Iteration**: It iterates through multiple turns until it has a sufficient recipe pool to satisfy all constraints.

### Output
A structured JSON object representing the plan strategy and recipe selections.

---

## 3. Plan Reviewer Agent

### Purpose
An autonomous agent responsible for revising existing meal plans based on user feedback. It shares the same "Pull" model as the Analyst, fetching replacement recipes dynamically via tools.

### Interaction Logic
1.  **Diff Analysis**: Compares the current plan against the user's adjustment request (e.g., "Make Monday vegetarian").
2.  **Autonomous Search**: Searches for specific replacements that fit the requested change while maintaining the rest of the plan's integrity.
3.  **Consistency Check**: Ensures batch-cooking patterns (Cook/Reuse) are preserved during the revision.

---

## 4. Chef Agent