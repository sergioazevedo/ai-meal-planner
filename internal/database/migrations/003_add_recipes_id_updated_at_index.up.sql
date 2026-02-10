-- 003_add_recipes_id_updated_at_index.up.sql
CREATE INDEX idx_recipes_id_updated_at ON recipes(id, updated_at);
