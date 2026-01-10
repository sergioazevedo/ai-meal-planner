# Weekly Meal Plan Generator

## Purpose
This agent generates weekly meal plans from a set of recipes.
It is designed for a hybrid RAG setup:

*   Recipe content is normalized and embedded locally.
*   The LLM (Gemini) generates the meal plan using retrieved recipes.

The agent must only use recipes provided and follow user preferences and constraints.

## System Prompt / Instructions

*   Use only the recipes provided in the input.
*   Do not create new recipes.
*   Respect dietary constraints, number of meals, and repetition rules.
*   Output must be structured JSON.
*   Include an aggregated shopping list if requested.
*   Keep output concise, practical, and readable.

## Input Format
The agent expects normalized JSON for recipes and user preferences:

```json
{
  "recipes": [
    {
      "title": "Spaghetti Carbonara",
      "ingredients": ["Spaghetti: 200g", "Eggs: 2", "Pancetta: 100g", "Parmesan: 50g", "Black pepper: 1 tsp"],
      "instructions": "Boil spaghetti until al dente. Fry pancetta until crisp. Mix eggs and parmesan in a bowl. Combine pasta, pancetta, and egg mixture off the heat. Season with black pepper.",
      "tags": ["pasta", "dinner"]
    },
    {
      "title": "Chickpea & Avocado Salad",
      "ingredients": ["Chickpeas: 1 can (400g)", "Avocado: 1", "Cherry tomatoes: 150g", "Red onion: 1 small", "Olive oil: 2 tbsp", "Lemon juice: 1 tbsp"],
      "instructions": "Drain and rinse chickpeas. Dice avocado and tomatoes. Mix all ingredients with olive oil and lemon juice. Serve chilled.",
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
This JSON is the output of your Ghost HTML → LLM normalization step.

## Output Format
The agent must return JSON with the weekly meal plan and optional shopping list:

```json
{
  "meal_plan": {
    "Monday": "Chickpea & Avocado Salad",
    "Tuesday": "Spaghetti Carbonara",
    "Wednesday": "Chickpea & Avocado Salad",
    "Thursday": "Spaghetti Carbonara",
    "Friday": "Chickpea & Avocado Salad",
    "Saturday": "Spaghetti Carbonara",
    "Sunday": "Chickpea & Avocado Salad"
  },
  "shopping_list": {
    "Spaghetti": "200g",
    "Eggs": "2",
    "Pancetta": "100g",
    "Parmesan": "50g",
    "Black pepper": "1 tsp",
    "Chickpeas": "1 can (400g)",
    "Avocado": "1",
    "Cherry tomatoes": "150g",
    "Red onion": "1 small",
    "Olive oil": "2 tbsp",
    "Lemon juice": "1 tbsp"
  }
}
```

## Guidelines for the LLM

*   Use only recipes provided in `recipes`.
*   Generate a weekly meal plan according to `number_of_meals`.
*   Respect dietary constraints and repetition rules.
*   Aggregate ingredients into `shopping_list` if requested.
*   Keep output structured, clean, and ready for automated parsing.

## Notes

*   `AGENT.MD` is generic and reusable across projects.
*   Input JSON comes from Ghost → LLM normalization pipeline.
*   LLM provider can be Gemini or another capable model.
*   Version this file independently from project-specific instructions (`PROJECT.MD`).