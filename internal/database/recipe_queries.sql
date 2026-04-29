-- name: InsertRecipe :exec
INSERT INTO recipes (id, data, updated_at)
VALUES (?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    data = EXCLUDED.data,
    updated_at = EXCLUDED.updated_at
WHERE EXCLUDED.updated_at > recipes.updated_at;

-- name: GetRecipeByID :one
SELECT id, data, updated_at FROM recipes
WHERE id = ?;

-- name: GetRecipesByIDs :many
SELECT id, data, updated_at FROM recipes
WHERE id IN (sqlc.slice('ids'));

-- name: ListRecipes :many
SELECT id, data, updated_at FROM recipes
WHERE id NOT IN (sqlc.slice('exclude_ids'))
ORDER BY updated_at DESC;

-- name: ListAllRecipes :many
SELECT id, data, updated_at FROM recipes
ORDER BY updated_at DESC;

-- name: DeleteRecipeByID :exec
DELETE FROM recipes WHERE id = ?;

-- name: GetRandomRecipes :many
SELECT id, data, updated_at FROM recipes
WHERE id NOT IN (sqlc.slice('exclude_ids'))
ORDER BY RANDOM()
LIMIT ?;

-- name: InsertRecipeTag :exec
INSERT INTO recipe_tags (recipe_id, tag)
VALUES (?, ?)
ON CONFLICT (recipe_id, tag) DO NOTHING;

-- name: DeleteRecipeTags :exec
DELETE FROM recipe_tags
WHERE recipe_id = ?;

-- name: GetRecipeIDsByTags :many
SELECT DISTINCT recipe_id
FROM recipe_tags
WHERE tag IN (sqlc.slice('tags'));
