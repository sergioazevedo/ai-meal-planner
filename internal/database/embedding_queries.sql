-- name: InsertEmbedding :exec
INSERT INTO recipe_embeddings (
    recipe_id,
    embedding,
    text_hash,
    embedding_model,
    embedding_dimensions
)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (recipe_id) DO UPDATE SET
    embedding = EXCLUDED.embedding,
    text_hash = EXCLUDED.text_hash,
    embedding_model = EXCLUDED.embedding_model,
    embedding_dimensions = EXCLUDED.embedding_dimensions;

-- name: GetEmbeddingByRecipeID :one
SELECT recipe_id, embedding, text_hash, embedding_model, embedding_dimensions
FROM recipe_embeddings
WHERE recipe_id = ?;

-- name: ListAllEmbeddings :many
SELECT recipe_id, embedding FROM recipe_embeddings;

-- name: DeleteEmbeddingByRecipeID :exec
DELETE FROM recipe_embeddings WHERE recipe_id = ?;
