-- name: InsertMealPlan :exec
INSERT INTO user_meal_plans (user_id, plan_data, created_at)
VALUES (?, ?, ?);

-- name: ListRecentMealPlansByUserID :many
SELECT id, user_id, plan_data, created_at FROM user_meal_plans
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: DeleteOldMealPlansByUserID :exec
DELETE FROM user_meal_plans
WHERE user_id = ? AND created_at < ?;
