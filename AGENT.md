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
  "tags": ["tag1", "tag2"]
}
```

---

## 2. Meal Plan Agent

### Purpose
To generate a weekly meal plan based on a provided list of structured recipes and a set of user preferences.

### Input
A JSON object containing a list of `recipes` (in the normalized format) and `user_preferences`.

```json
{
  "recipes": [
    {
      "title": "Spaghetti Carbonara",
      "ingredients": ["..."],
      "instructions": "...",
      "tags": ["pasta", "dinner"]
    },
    {
      "title": "Chickpea & Avocado Salad",
      "ingredients": ["..."],
      "instructions": "...",
      "tags": ["salad", "vegetarian", "lunch"]
    }
  ],
  "user_preferences": {
    "diet": "Vegetarian",
    "number_of_meals": 7,
    "avoid_repeating_recipes": true,
    "include_shopping_list": true
  }
}
```

### Output
A JSON object containing the `meal_plan` and an optional aggregated `shopping_list`.

```json
{
  "meal_plan": {
    "Monday": "Recipe Name",
    "Tuesday": "Another Recipe"
  },
  "shopping_list": {
    "Ingredient A": "Total Amount",
    "Ingredient B": "Total Amount"
  }
}
```