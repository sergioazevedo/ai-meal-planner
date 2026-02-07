package planner

import (
	"ai-meal-planner/internal/shared"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"time"
)

//go:embed chef_prompt.md
var chefPrompt string

type ChefResult struct {
	Plan *MealPlan
	Meta shared.AgentMeta
}

func (p *Planner) runChef(
	ctx context.Context,
	mealSchedule *MealProposal,
	weekStart time.Time,
) (ChefResult, error) {
	start := time.Now()
	prompt, err := buildChefPrompt(mealSchedule)
	if err != nil {
		return ChefResult{}, err
	}

	resp, err := p.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return ChefResult{}, err
	}

	result := &MealPlan{}
	if err = json.Unmarshal([]byte(resp.Content), result); err != nil {
		return ChefResult{
				Meta: shared.AgentMeta{
					Usage:     resp.Usage,
					AgentName: "Chef",
				}},
			fmt.Errorf(
				"failed to parse MealPlan %w, :%s",
				err,
				resp.Content,
			)
	}

	result.WeekStart = weekStart
	return ChefResult{
		Plan: result,
		Meta: shared.AgentMeta{
			AgentName: "Chef",
			Usage:     resp.Usage,
			Latency:   time.Since(start),
		},
	}, nil
}

func buildChefPrompt(data *MealProposal) (string, error) {
	tmpl, err := template.New("Chef").Parse(chefPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
