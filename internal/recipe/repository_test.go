package recipe

import (
	"context"
	"path/filepath"
	"testing"

	"ai-meal-planner/internal/database"
	"ai-meal-planner/internal/value"
)

func TestRepositoryGetByIdsPreservesRequestedOrder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "recipes.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	defer db.Close()

	if err := db.MigrateUp(dbPath); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	repo := NewRepository(db.SQL)
	ctx := context.Background()
	for _, rec := range []value.Recipe{
		{ID: "a", Title: "First", UpdatedAt: "2023-01-01T00:00:00Z"},
		{ID: "b", Title: "Second", UpdatedAt: "2023-01-01T00:00:00Z"},
		{ID: "c", Title: "Third", UpdatedAt: "2023-01-01T00:00:00Z"},
	} {
		if err := repo.Save(ctx, rec); err != nil {
			t.Fatalf("save recipe %s: %v", rec.ID, err)
		}
	}

	recipes, err := repo.GetByIds(ctx, []string{"c", "a", "b"})
	if err != nil {
		t.Fatalf("get recipes: %v", err)
	}

	if len(recipes) != 3 {
		t.Fatalf("got %d recipes, want 3", len(recipes))
	}
	for i, wantID := range []string{"c", "a", "b"} {
		if recipes[i].ID != wantID {
			t.Errorf("recipes[%d].ID = %q, want %q", i, recipes[i].ID, wantID)
		}
	}
}
