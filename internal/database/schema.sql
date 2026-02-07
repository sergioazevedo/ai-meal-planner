-- internal/database/schema.sql

-- Existing execution_metrics table, moved here for consistency with sqlc
CREATE TABLE IF NOT EXISTS execution_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_tokens INTEGER NOT NULL,
    completion_tokens INTEGER NOT NULL,
    latency_ms INTEGER NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_execution_metrics_timestamp ON execution_metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_execution_metrics_agent_model ON execution_metrics(agent_name, model);


-- recipes table (document store approach)
CREATE TABLE IF NOT EXISTS recipes (
    id TEXT PRIMARY KEY NOT NULL,         -- Unique recipe ID
    data TEXT NOT NULL,                   -- Full recipe JSON as text
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_recipes_updated_at ON recipes(updated_at);
-- Optional: Index on frequently queried JSON fields if performance becomes an issue
-- e.g., CREATE INDEX IF NOT EXISTS idx_recipes_title ON recipes(json_extract(data, '$.title'));


-- recipe_embeddings table
CREATE TABLE IF NOT EXISTS recipe_embeddings (
    recipe_id TEXT PRIMARY KEY NOT NULL,  -- Foreign key to recipes.id
    embedding BLOB NOT NULL,              -- Serialized []float32 vector
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);
-- No additional index needed for recipe_id as it's the PRIMARY KEY


-- user_meal_plans table (user memory)
CREATE TABLE IF NOT EXISTS user_meal_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,                -- Identifier for the user (e.g., Telegram Chat ID)
    plan_data TEXT NOT NULL               -- Full meal plan JSON as text
);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_user_id ON user_meal_plans(user_id);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_user_id_id ON user_meal_plans(user_id, id DESC);
