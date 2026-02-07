-- name: InsertEmbedding :exec
INSERT INTO recipe_embeddings (recipe_id, embedding)
VALUES (?, ?)
ON CONFLICT (recipe_id) DO UPDATE SET
    embedding = EXCLUDED.embedding;

-- name: GetEmbeddingByRecipeID :one
SELECT recipe_id, embedding FROM recipe_embeddings
WHERE recipe_id = ?;

-- name: ListAllEmbeddings :many
SELECT recipe_id, embedding FROM recipe_embeddings;

-- name: DeleteEmbeddingByRecipeID :exec
DELETE FROM recipe_embeddings WHERE recipe_id = ?;
