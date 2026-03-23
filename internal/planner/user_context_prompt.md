### User Context
User Request: "{{ .UserRequest }}"
Household: {{ .Adults }} Adults, {{ .Children }} Children (Ages: {{ .ChildrenAges }})

### Initial Recipe Suggestions
{{ range .Recipes }}
- {{ .Title }} | Tags: {{ range .Tags }}{{ . }}, {{ end }} | Time: {{ .PrepTime }} | Serves: {{ .Servings }}
{{ end }}

If you can build a perfect 5-recipe plan from the suggestions above, do it immediately. IF AND ONLY IF these do not meet the constraints or you need more variety, use the `search_recipes` tool to find alternatives.
