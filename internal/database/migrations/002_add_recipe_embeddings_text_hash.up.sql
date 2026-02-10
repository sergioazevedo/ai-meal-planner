-- 002_add_recipe_embeddings_text_hash.up.sql
ALTER TABLE recipe_embeddings ADD COLUMN text_hash TEXT NOT NULL DEFAULT '';
