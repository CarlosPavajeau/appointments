-- name: DeleteScheduleOverride :exec
DELETE
FROM schedule_overrides
WHERE id = $1
  AND resource_id = $2;