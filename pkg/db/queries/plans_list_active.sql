-- name: ListActivePlans :many
SELECT id,
       external_id,
       name,
       description,
       price,
       currency,
       "interval"
FROM plans
WHERE is_active = true
  AND environment = $1;