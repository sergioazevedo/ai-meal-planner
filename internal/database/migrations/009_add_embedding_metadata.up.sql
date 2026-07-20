ALTER TABLE recipe_embeddings
ADD COLUMN embedding_model TEXT NOT NULL DEFAULT '';

ALTER TABLE recipe_embeddings
ADD COLUMN embedding_dimensions INTEGER NOT NULL DEFAULT 0;
