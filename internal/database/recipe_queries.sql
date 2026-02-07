-- name: InsertRecipe :exec
INSERT INTO recipes (id, data, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    data = EXCLUDED.data,
    updated_at = EXCLUDED.updated_at;

-- name: GetRecipeByID :one
SELECT id, data, created_at, updated_at FROM recipes
WHERE id = ?;

-- name: ListRecipes :many
SELECT id, data, created_at, updated_at FROM recipes
ORDER BY created_at DESC;

-- name: DeleteRecipeByID :exec
DELETE FROM recipes WHERE id = ?;

-- name: CountRecipes :one
SELECT COUNT(id) FROM recipes;

