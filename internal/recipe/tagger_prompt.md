# Recipe Tagger

Generate accurate search tags for the normalized recipe below.

## Recipe

Title: {{ .Title }}
Ingredients: {{ .IngredientsJSON }}
Source tags: {{ .SourceTagsJSON }}

## Rules

- Return each tag concept as an explicit Brazilian Portuguese (`pt`) and English (`en`) translation pair.
- Both values in a pair MUST express the same concept. For example, `salmão` pairs with `salmon`, not `fish`.
- Include the main ingredients, useful meal categories, and relevant dietary categories.
- Preserve useful source-tag concepts, but translate them into both languages.
- Never label a recipe vegetarian or vegan when it contains fish, meat, poultry, seafood, or another incompatible ingredient.
- Do not invent ingredients or dietary properties.
- Use lowercase, concise tags.
- Return raw JSON only, without markdown.

## Output

{
  "tags": [
    {"pt": "salmão", "en": "salmon"},
    {"pt": "brócolis", "en": "broccoli"}
  ]
}
