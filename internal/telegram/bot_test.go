package telegram

import (
	"strings"
	"testing"

	"ai-meal-planner/internal/planner"
)

func TestFormatPlanMarkdownParts(t *testing.T) {
	plan := &planner.MealPlan{
		Plan: []planner.DayPlan{
			{Day: "Monday", RecipeTitle: "Tacos", PrepTime: "15 mins", Note: "Tasty"},
			{Day: "Tuesday", RecipeTitle: "Salad", PrepTime: "10 mins", Note: ""},
		},
		ShoppingList: []string{"Cheese", "Lettuce"},
	}

	planOutput, shoppingOutput := formatPlanMarkdownParts(plan)

	// Check Plan Header
	if !strings.Contains(planOutput, "📅 *Weekly Meal Plan*") {
		t.Error("Missing plan header")
	}

	// Check Days and individual prep times
	if !strings.Contains(planOutput, "*Monday*: Tacos (15 mins)") {
		t.Error("Missing Monday plan or prep time")
	}
	if !strings.Contains(planOutput, "_Tasty_") {
		t.Error("Missing note for Monday")
	}

	// Check Shopping List Header
	if !strings.Contains(shoppingOutput, "🛒 *Shopping List*") {
		t.Error("Missing shopping list header")
	}
	if !strings.Contains(shoppingOutput, "• Cheese") {
		t.Error("Missing shopping item")
	}

	// Check Total Prep Time
	if !strings.Contains(planOutput, "⏱ *Total Prep:* 25 mins") {
		t.Error("Missing total prep time")
	}
}
