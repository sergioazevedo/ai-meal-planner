-- Consolidated schema for sqlc code generation
-- This file should reflect all applied migrations

-- execution_metrics table
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
    id TEXT PRIMARY KEY NOT NULL,
    data TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_recipes_updated_at ON recipes(updated_at);
CREATE INDEX IF NOT EXISTS idx_recipes_id_updated_at ON recipes(id, updated_at);

-- recipe_embeddings table
CREATE TABLE IF NOT EXISTS recipe_embeddings (
    recipe_id TEXT PRIMARY KEY NOT NULL,
    embedding BLOB NOT NULL,
    text_hash TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (recipe_id) REFERENCES recipes(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_recipe_embeddings_text_hash ON recipe_embeddings(text_hash);

-- user_meal_plans table (user memory)
CREATE TABLE IF NOT EXISTS user_meal_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    plan_data TEXT NOT NULL,
    week_start_date DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'FINAL',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_user_id ON user_meal_plans(user_id);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_week_start ON user_meal_plans(week_start_date);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_user_id_week ON user_meal_plans(user_id, week_start_date DESC);
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_status ON user_meal_plans(status);

-- shopping_lists table
CREATE TABLE IF NOT EXISTS shopping_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    meal_plan_id INTEGER NOT NULL,
    items TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY (meal_plan_id) REFERENCES user_meal_plans(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_shopping_lists_user_id ON shopping_lists(user_id);
CREATE INDEX IF NOT EXISTS idx_shopping_lists_meal_plan_id ON shopping_lists(meal_plan_id);
CREATE INDEX IF NOT EXISTS idx_shopping_lists_user_plan ON shopping_lists(user_id, meal_plan_id);

-- user_sessions table (for tracking conversation state during plan adjustments)
CREATE TABLE IF NOT EXISTS user_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    session_type TEXT NOT NULL,
    state TEXT NOT NULL,
    context_data TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active ON user_sessions(user_id, expires_at);
