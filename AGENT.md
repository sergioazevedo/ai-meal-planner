# Project Agents

This document describes the conceptual AI agents used in the Meal Planner project. An agent is a logical component that uses a Large Language Model (LLM) to perform a specific reasoning task.

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

## 2. Meal Plan Agent

### Purpose
To generate a weekly meal plan based on a provided list of structured recipes and a set of user preferences.

### Input
A text prompt containing the User's Request and a context block of relevant recipes retrieved via semantic search.

```text
User Request: "Healthy vegetarian dinners"

Available Recipes:
Recipe 1:
Title: ...
...
```

### Output
A JSON object containing the `plan` (list of daily assignments) and an aggregated `shopping_list`.

```json
{
  "plan": [
    {
      "day": "Monday",
      "recipe_title": "Recipe Name",
      "note": "Chosen because it is quick to make"
    },
    ...
  ],
  "shopping_list": ["Ingredient 1", "Ingredient 2"],
  "total_prep_estimate": "3 hours"
}
```
