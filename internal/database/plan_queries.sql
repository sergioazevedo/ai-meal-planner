-- name: InsertMealPlan :exec
INSERT INTO user_meal_plans (user_id, plan_data, week_start_date, created_at)
VALUES (?, ?, ?, ?);

-- name: ListRecentMealPlansByUserID :many
SELECT id, user_id, plan_data, week_start_date, created_at FROM user_meal_plans
WHERE user_id = ?
ORDER BY week_start_date DESC
LIMIT ?;

-- name: DeleteOldMealPlansByUserID :exec
DELETE FROM user_meal_plans
WHERE user_id = ? AND week_start_date < ?;

-- name: CheckPlanExists :one
SELECT COUNT(*) FROM user_meal_plans
WHERE user_id = ? AND week_start_date = ?;
