-- name: InsertMealPlan :one
INSERT INTO user_meal_plans (user_id, plan_data, week_start_date, status, created_at)
VALUES (?, ?, ?, ?, ?)
RETURNING id;

-- name: ListRecentMealPlansByUserID :many
SELECT id, user_id, plan_data, week_start_date, status, created_at FROM user_meal_plans
WHERE user_id = ?
ORDER BY week_start_date DESC
LIMIT ?;

-- name: DeleteOldMealPlansByUserID :exec
DELETE FROM user_meal_plans
WHERE user_id = ? AND week_start_date < ?;

-- name: CheckPlanExists :one
SELECT COUNT(*) FROM user_meal_plans
WHERE user_id = ? AND week_start_date = ?;

-- name: GetMealPlanByID :one
SELECT id, user_id, plan_data, week_start_date, status, created_at FROM user_meal_plans
WHERE id = ?;

-- name: GetDraftPlanByUserAndWeek :one
SELECT id, user_id, plan_data, week_start_date, status, created_at FROM user_meal_plans
WHERE user_id = ? AND week_start_date = ? AND status = 'DRAFT'
LIMIT 1;

-- name: UpdatePlanStatus :exec
UPDATE user_meal_plans
SET status = ?
WHERE id = ?;
