-- name: FindResourceScheduleOverrides :many
SELECT id,
       resource_id,
       date,
       is_day_off,
       start_time::text,
       end_time::text,
       COALESCE(reason, '') as reason,
       created_at
FROM schedule_overrides
WHERE resource_id = $1
  AND date BETWEEN $2 AND $3
ORDER BY date;