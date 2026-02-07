package recipe

import (
	db "ai-meal-planner/internal/recipe/db"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository is a database-backed repository for recipes.
type Repository struct {
	queries *db.Queries
	db      *sql.DB // Direct database access for transactions if needed
}

// NewRepository creates a new Repository.
func NewRepository(d *sql.DB) *Repository {
	return &Repository{
		queries: db.New(d),
		db:      d,
	}
}

// Save inserts or updates a recipe in the database.
func (r *Repository) Save(ctx context.Context, rec Recipe) error {
	recipeJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe to JSON: %w", err)
	}

	var dbUpdatedAt time.Time
	if rec.UpdatedAt != "" {
		// Attempt to parse the string timestamp from JSON, assuming RFC3339.
		parsedTime, err := time.Parse(time.RFC3339, rec.UpdatedAt)
		if err != nil {
			// Log a warning and use current time for DB update if parsing fails.
			fmt.Printf("Warning: Failed to parse rec.UpdatedAt '%s' for recipe %s: %v. Using current time for DB update.\n", rec.UpdatedAt, rec.ID, err)
			dbUpdatedAt = time.Now()
		} else {
			dbUpdatedAt = parsedTime
		}
	} else {
		// If source_updated_at is empty, use current time for DB update.
		dbUpdatedAt = time.Now()
	}

	params := db.InsertRecipeParams{
		ID:        rec.ID,
		Data:      string(recipeJSON),
		UpdatedAt: dbUpdatedAt, // Use the determined time
	}

	return r.queries.InsertRecipe(ctx, params)
}

// Get retrieves a recipe by its ID.
func (r *Repository) Get(ctx context.Context, id string) (*Recipe, error) {
	dbRecipe, err := r.queries.GetRecipeByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Recipe not found
		}
		return nil, fmt.Errorf("failed to get recipe by ID: %w", err)
	}

	var rec Recipe
	if err := json.Unmarshal([]byte(dbRecipe.Data), &rec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal recipe JSON: %w", err)
	}

	return &rec, nil
}

// GetByIds retrieves multiple recipes by their IDs.
func (r *Repository) GetByIds(ctx context.Context, ids []string) ([]Recipe, error) {
	dbRecipes, err := r.queries.GetRecipesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipes by IDs: %w", err)
	}

	var recipes []Recipe
	for _, dbRec := range dbRecipes {
		var rec Recipe
		if err := json.Unmarshal([]byte(dbRec.Data), &rec); err != nil {
			fmt.Printf("Warning: Failed to unmarshal recipe JSON for ID %s: %v\n", dbRec.ID, err)
			continue
		}
		recipes = append(recipes, rec)
	}
	return recipes, nil
}

// List retrieves all recipes, optionally excluding specified IDs.
func (r *Repository) List(ctx context.Context, excludeIDs []string) ([]Recipe, error) {
	var dbRecipes []db.Recipe
	var err error

	if len(excludeIDs) > 0 {
		dbRecipes, err = r.queries.ListRecipes(ctx, excludeIDs)
	} else {
		dbRecipes, err = r.queries.ListAllRecipes(ctx)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list recipes: %w", err)
	}

	var recipes []Recipe
	for _, dbRec := range dbRecipes {
		var rec Recipe
		if err := json.Unmarshal([]byte(dbRec.Data), &rec); err != nil {
			// Log error and skip invalid recipe, or return error for corrupted data
			fmt.Printf("Warning: Failed to unmarshal recipe JSON for ID %s: %v\n", dbRec.ID, err)
			continue
		}
		// ID populated from JSON.
		recipes = append(recipes, rec)
	}
	return recipes, nil
}

// Count returns the number of recipes in the database.
func (r *Repository) Count(ctx context.Context) (int, error) {
	count, err := r.queries.CountRecipes(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count recipes: %w", err)
	}
	return int(count), nil
}
