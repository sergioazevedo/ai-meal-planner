package planner

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
)

//go:embed chef_prompt.md
var chefPrompt string

type ChefResult struct {
	Plan *MealPlan
	Meta AgentMeta
}

func (p *Planner) runChef(
	ctx context.Context,
	mealSchedule *MealProposal,
) (ChefResult, error) {
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
		return ChefResult{Meta: AgentMeta{Usage: resp.Usage}}, fmt.Errorf("failed to parse MealPlan %w, :%s", err, resp.Content)
	}

	return ChefResult{
		Plan: result,
		Meta: AgentMeta{Usage: resp.Usage},
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
