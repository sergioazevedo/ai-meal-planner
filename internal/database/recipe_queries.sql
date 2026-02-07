-- name: InsertRecipe :exec
INSERT OR REPLACE INTO recipes (id, data, created_at, updated_at)
VALUES (?, ?, ?, ?);

-- name: GetRecipeByID :one
SELECT id, data, created_at, updated_at FROM recipes
WHERE id = ?;

-- name: ListRecipes :many
SELECT id, data, created_at, updated_at FROM recipes
ORDER BY created_at DESC;

-- name: DeleteRecipeByID :exec
DELETE FROM recipes WHERE id = ?;
