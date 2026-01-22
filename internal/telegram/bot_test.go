package telegram

import (
	"strings"
	"testing"

	"ai-meal-planner/internal/planner"
)

func TestFormatPlanMarkdown(t *testing.T) {
	plan := &planner.MealPlan{
		Plan: []planner.DayPlan{
			{Day: "Monday", RecipeTitle: "Tacos", Note: "Tasty"},
			{Day: "Tuesday", RecipeTitle: "Salad", Note: ""},
		},
		ShoppingList: []string{"Cheese", "Lettuce"},
		TotalPrep:    "30 mins",
	}

	output := formatPlanMarkdown(plan)

	// Check Header
	if !strings.Contains(output, "📅 *Weekly Meal Plan*") {
		t.Error("Missing header")
	}

	// Check Days
	if !strings.Contains(output, "*Monday*: Tacos") {
		t.Error("Missing Monday plan")
	}
	if !strings.Contains(output, "_Tasty_") {
		t.Error("Missing note for Monday")
	}

	// Check Shopping List
	if !strings.Contains(output, "🛒 *Shopping List*") {
		t.Error("Missing shopping list header")
	}
	if !strings.Contains(output, "• Cheese") {
		t.Error("Missing shopping item")
	}

	// Check Prep Time
	if !strings.Contains(output, "⏱ *Total Prep:* 30 mins") {
		t.Error("Missing prep time")
	}
}
