-- name: InsertShoppingList :one
INSERT INTO shopping_lists (user_id, meal_plan_id, items, created_at)
VALUES (?, ?, ?, ?)
RETURNING id;

-- name: GetShoppingListByMealPlanID :one
SELECT id, user_id, meal_plan_id, items, created_at FROM shopping_lists
WHERE meal_plan_id = ?
LIMIT 1;

-- name: GetShoppingListByUserAndWeek :one
SELECT sl.id, sl.user_id, sl.meal_plan_id, sl.items, sl.created_at
FROM shopping_lists sl
INNER JOIN user_meal_plans ump ON sl.meal_plan_id = ump.id
WHERE sl.user_id = ? AND ump.week_start_date = ?
LIMIT 1;

-- name: DeleteShoppingListByMealPlanID :exec
DELETE FROM shopping_lists
WHERE meal_plan_id = ?;
