-- name: InsertRecipe :exec
INSERT INTO recipes (id, data, updated_at)
VALUES (?, ?, ?);

-- name: GetRecipeByID :one
SELECT id, data, updated_at FROM recipes
WHERE id = ?;

-- name: GetRecipesByIDs :many
SELECT id, data, updated_at FROM recipes
WHERE id IN (sqlc.slice('ids'));


-- name: ListRecipes :many
SELECT id, data, updated_at FROM recipes
ORDER BY updated_at DESC;

-- name: DeleteRecipeByID :exec
DELETE FROM recipes WHERE id = ?;

-- name: CountRecipes :one
SELECT COUNT(id) FROM recipes;
