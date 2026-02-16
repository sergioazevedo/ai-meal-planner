package shopping

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	shoppingdb "ai-meal-planner/internal/shopping/db"
)

// Repository handles persistence of shopping lists.
type Repository struct {
	queries *shoppingdb.Queries
	db      *sql.DB
}

// NewRepository creates a new shopping list repository.
func NewRepository(d *sql.DB) *Repository {
	return &Repository{
		queries: shoppingdb.New(d),
		db:      d,
	}
}

// Save creates a new shopping list in the database.
func (r *Repository) Save(ctx context.Context, list *ShoppingList) (int64, error) {
	itemsJSON, err := json.Marshal(list.Items)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal shopping list items: %w", err)
	}

	id, err := r.queries.InsertShoppingList(ctx, shoppingdb.InsertShoppingListParams{
		UserID:     list.UserID,
		MealPlanID: list.MealPlanID,
		Items:      string(itemsJSON),
		CreatedAt:  time.Now().UTC(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert shopping list: %w", err)
	}

	return id, nil
}

// GetByMealPlanID retrieves a shopping list by meal plan ID.
func (r *Repository) GetByMealPlanID(ctx context.Context, mealPlanID int64) (*ShoppingList, error) {
	dbList, err := r.queries.GetShoppingListByMealPlanID(ctx, mealPlanID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No shopping list found
		}
		return nil, fmt.Errorf("failed to get shopping list by meal plan ID: %w", err)
	}

	var items []string
	if err := json.Unmarshal([]byte(dbList.Items), &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shopping list items: %w", err)
	}

	return &ShoppingList{
		ID:         dbList.ID,
		UserID:     dbList.UserID,
		MealPlanID: dbList.MealPlanID,
		Items:      items,
		CreatedAt:  dbList.CreatedAt,
	}, nil
}

// GetByUserAndWeek retrieves a shopping list by user ID and week start date.
func (r *Repository) GetByUserAndWeek(ctx context.Context, userID string, weekStart time.Time) (*ShoppingList, error) {
	dbList, err := r.queries.GetShoppingListByUserAndWeek(ctx, shoppingdb.GetShoppingListByUserAndWeekParams{
		UserID:        userID,
		WeekStartDate: weekStart,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No shopping list found
		}
		return nil, fmt.Errorf("failed to get shopping list by user and week: %w", err)
	}

	var items []string
	if err := json.Unmarshal([]byte(dbList.Items), &items); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shopping list items: %w", err)
	}

	return &ShoppingList{
		ID:         dbList.ID,
		UserID:     dbList.UserID,
		MealPlanID: dbList.MealPlanID,
		Items:      items,
		CreatedAt:  dbList.CreatedAt,
	}, nil
}

// DeleteByMealPlanID deletes a shopping list by meal plan ID.
func (r *Repository) DeleteByMealPlanID(ctx context.Context, mealPlanID int64) error {
	return r.queries.DeleteShoppingListByMealPlanID(ctx, mealPlanID)
}
