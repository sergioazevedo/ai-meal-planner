-- name: InsertMealPlan :exec
INSERT INTO user_meal_plans (user_id, plan_data)
VALUES (?, ?);

-- name: ListRecentMealPlansByUserID :many
SELECT id, user_id, plan_data FROM user_meal_plans
WHERE user_id = ?
ORDER BY id DESC
LIMIT ?;

-- name: DeleteOldMealPlansByUserID :exec
DELETE FROM user_meal_plans
WHERE user_id = ? AND id < ?;
