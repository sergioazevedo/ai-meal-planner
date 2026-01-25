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

func (p *Planner) runChef(
	ctx context.Context,
	mealSchedule *MealProposal,
) (*MealPlan, error) {
	prompt, err := buildChefPrompt(mealSchedule)
	if err != nil {
		return nil, err
	}

	llmResp, err := p.textGen.GenerateContent(ctx, prompt)
	if err != nil {
		return nil, err
	}

	result := &MealPlan{}
	if err = json.Unmarshal([]byte(llmResp), result); err != nil {
		return nil, fmt.Errorf("failed to parse MealPlan %w, :%s", err, llmResp)
	}

	return result, nil
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
