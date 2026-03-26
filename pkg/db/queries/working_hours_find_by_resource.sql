-- name: FindResourceWorkingHours :many
SELECT id,
       resource_id,
       day_of_week,
       start_time::text,
       end_time::text,
       is_active
FROM working_hours
WHERE resource_id = $1
ORDER BY day_of_week;