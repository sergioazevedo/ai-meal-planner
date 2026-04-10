-- 004_add_plan_status_and_shopping_lists.up.sql

-- Add status column to user_meal_plans table
ALTER TABLE user_meal_plans ADD COLUMN status TEXT NOT NULL DEFAULT 'FINAL';

-- Create index on status for efficient querying
CREATE INDEX IF NOT EXISTS idx_user_meal_plans_status ON user_meal_plans(status);

-- Create shopping_lists table
CREATE TABLE IF NOT EXISTS shopping_lists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    meal_plan_id INTEGER NOT NULL,
    items TEXT NOT NULL,                   -- JSON array of shopping list items
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY (meal_plan_id) REFERENCES user_meal_plans(id) ON DELETE CASCADE
);

-- Create indexes for shopping_lists
CREATE INDEX IF NOT EXISTS idx_shopping_lists_user_id ON shopping_lists(user_id);
CREATE INDEX IF NOT EXISTS idx_shopping_lists_meal_plan_id ON shopping_lists(meal_plan_id);
CREATE INDEX IF NOT EXISTS idx_shopping_lists_user_plan ON shopping_lists(user_id, meal_plan_id);
