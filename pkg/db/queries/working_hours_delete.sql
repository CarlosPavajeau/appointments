-- name: DeleteWorkingHour :exec
DELETE
FROM working_hours
WHERE id = $1
  AND resource_id = $2;