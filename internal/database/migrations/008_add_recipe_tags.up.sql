CREATE TABLE IF NOT EXISTS recipe_tags (
    recipe_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    PRIMARY KEY (recipe_id, tag),
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_recipe_tags_tag ON recipe_tags(tag);
