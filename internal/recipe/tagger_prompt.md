# Recipe Tagger

Generate accurate search tags for the normalized recipe below.

## Recipe

Title: {{ .Title }}
Ingredients: {{ .IngredientsJSON }}
Source tags: {{ .SourceTagsJSON }}

## Rules

- Return each tag concept as an explicit Brazilian Portuguese (`pt-BR`) and English (`en`) translation pair.
- The `pt-BR` value MUST be Brazilian Portuguese and the `en` value MUST be English. Never swap the fields and never use Spanish.
- Both values in a pair MUST express the same concept. For example, `salmão` pairs with `salmon`, not `fish`.
- Include the main ingredients and useful source-tag concepts.
- Preserve useful source-tag concepts, but translate them into both languages.
- Do not infer dietary labels such as vegetarian, vegan, low carb, or gluten free. Include one only when it is explicitly present in the source tags and compatible with the ingredients.
- Do not invent ingredients or dietary properties.
- Use lowercase, concise tags.
- Return raw JSON only, without markdown.

Correct: `{"pt-BR":"fritadeira sem óleo","en":"air fryer"}`
Forbidden: `{"pt-BR":"asador de ar","en":"air fryer"}` (Spanish)
Forbidden: `{"pt-BR":"low carb","en":"baixo carboidrato"}` (languages swapped)

## Output

{
  "tags": [
    {"pt-BR": "salmão", "en": "salmon"},
    {"pt-BR": "brócolis", "en": "broccoli"}
  ]
}
