-- 004_add_plan_status_and_shopping_lists.down.sql

-- Drop shopping_lists table and its indexes
DROP INDEX IF EXISTS idx_shopping_lists_user_plan;
DROP INDEX IF EXISTS idx_shopping_lists_meal_plan_id;
DROP INDEX IF EXISTS idx_shopping_lists_user_id;
DROP TABLE IF EXISTS shopping_lists;

-- Remove status column and index from user_meal_plans
DROP INDEX IF EXISTS idx_user_meal_plans_status;
-- SQLite doesn't support DROP COLUMN before 3.35.0, so we'd need to recreate the table
-- For safety, we'll document this as a non-reversible migration in production
-- In dev, you can just delete and recreate the database
