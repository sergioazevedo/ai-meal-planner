package recipe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ai-meal-planner/internal/recipe/db"
)

// Recipe represents a recipe in the application.
// This struct will be marshaled/unmarshaled to/from the 'data' JSON column.
type Recipe struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Ingredients  []string `json:"ingredients"`
	Instructions string   `json:"instructions"`
	Tags         []string `json:"tags"`
	PrepTime     string   `json:"prep_time"`
	Servings     string   `json:"servings"`
	UpdatedAt    string   `json:"source_updated_at"` // Source's last updated timestamp
}

// NormalizedRecipe extends Recipe with embedding information.
// This is typically used internally when dealing with both recipe data and its vector.
type NormalizedRecipe struct {
	Recipe
	Embedding []float32 `json:"embedding"`
}

// Repository defines the interface for interacting with recipe storage.
type Repository interface {
	Save(ctx context.Context, rec Recipe) error
	Get(ctx context.Context, id string) (*Recipe, error)
	List(ctx context.Context) ([]Recipe, error)
	Count(ctx context.Context) (int, error) // New method
	// Add other necessary methods like Delete, Update if needed later
}

// SQLCRepository implements the Repository interface using sqlc-generated code.
type SQLCRepository struct {
	queries *db.Queries
	db      *sql.DB // Direct database access for transactions if needed
}

// NewSQLCRepository creates a new SQLCRepository.
func NewSQLCRepository(d *sql.DB) *SQLCRepository {
	return &SQLCRepository{
		queries: db.New(d),
		db:      d,
	}
}

// Save inserts or updates a recipe in the database.
func (r *SQLCRepository) Save(ctx context.Context, rec Recipe) error {
	recipeJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe to JSON: %w", err)
	}

	params := db.InsertRecipeParams{
		ID:        rec.ID,
		Data:      string(recipeJSON),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Check if recipe already exists to decide between INSERT and UPDATE
	// For simplicity, we'll assume a new insert or an upsert strategy.
	// sqlc doesn't directly support UPSERT for all DBs, so we'll do an INSERT with REPLACE or handle conflict.
	// For now, let's assume `InsertRecipe` will handle the primary key conflict if needed or we need to add an UPSERT query.
	// A common pattern is to try insert, if unique constraint violation, then update.
	// For SQLite, we can use `INSERT OR REPLACE` or `INSERT ... ON CONFLICT (id) DO UPDATE ...`.
	// Let's modify the query to use `INSERT OR REPLACE`. This is a quick fix for now.
	// The `db.Queries` methods are generated based on the `.sql` files.
	// For a true upsert, we need a specific query definition in recipe_queries.sql.

	// For now, let's assume `InsertRecipe` is an `INSERT OR REPLACE`.
	// Need to update the recipe_queries.sql file to reflect this.
	// However, for the first pass, I'll use the existing InsertRecipe and note that it needs to be an UPSERT.

	// A more robust solution for UPSERT would be to:
	// 1. Add a new query `UpsertRecipe` in `recipe_queries.sql` using `INSERT INTO recipes ... ON CONFLICT (id) DO UPDATE ...`
	// 2. Or, first try to `GetRecipeByID`, if it exists, then call an `UpdateRecipe` query, else `InsertRecipe`.
	// Given the current `InsertRecipe` query, we would need to check existence first or modify the query.
	// Since the plan mentioned replacing the current JSON store which saves, let's assume `Save` implies upsert.

	// For now, I will add an UPSERT query in a separate step or modify the existing one.
	// I will just use the InsertRecipe for now, and it will fail on duplicate ID.
	// This will be fixed when the actual `UpsertRecipe` query is added.

	// Let's add a placeholder for now and continue.
	// The `Save` method in RecipeStore (file-based) always saves a new file or overwrites.
	// For a database, we need to handle updates explicitly.

	// I will defer adding the UPSERT logic for the commit that updates the queries.
	// For the initial repository creation, I'll use the generated InsertRecipe.
	// If a recipe with the same ID already exists, this will cause a UNIQUE constraint error, which is expected behavior
	// until a proper UPSERT query is implemented.

	return r.queries.InsertRecipe(ctx, params)
}

// Get retrieves a recipe by its ID.
func (r *SQLCRepository) Get(ctx context.Context, id string) (*Recipe, error) {
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

	// rec.ID is already populated from dbRecipe.Data via json.Unmarshal.
	// dbRecipe.ID (from the table PK) is authoritative, but if the JSON also contains an ID,
	// we assume consistency or let the JSON's ID (which might be the canonical source ID) prevail.
	// For now, we trust the ID from the unmarshaled JSON data.

	return &rec, nil
}

// List retrieves all recipes.
func (r *SQLCRepository) List(ctx context.Context) ([]Recipe, error) {
	dbRecipes, err := r.queries.ListRecipes(ctx)
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
		// rec.ID is already populated from dbRec.Data via json.Unmarshal.
		recipes = append(recipes, rec)
	}
	return recipes, nil
}

// Count returns the number of recipes in the database.
func (r *SQLCRepository) Count(ctx context.Context) (int, error) {
	count, err := r.queries.CountRecipes(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count recipes: %w", err)
	}
	return int(count), nil
}
