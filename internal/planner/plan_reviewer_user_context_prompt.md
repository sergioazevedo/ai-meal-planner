### User Context
Original User Request: "{{ .OriginalRequest }}"
Household: {{ .Adults }} Adults, {{ .Children }} Children (Ages: {{ .ChildrenAges }})

### Draft Meal Plan
{{ range .CurrentPlan }}
- **{{ .Day }}**: {{ .RecipeTitle }} ({{ .PrepTime }}){{ if .SideDishes }} — {{ range .SideDishes }}{{ . }}, {{ end }}{{ end }}{{ if .Note }} - _{{ .Note }}_{{ end }}
{{ end }}

## User Feedback/Adjustment Request
{{ .AdjustmentFeedback }}
