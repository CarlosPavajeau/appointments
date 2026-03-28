-- name: FindResourceOccupiedSlots :many
SELECT a.starts_at, a.ends_at, r.name AS resource_name
FROM appointments a
         INNER JOIN resources r ON a.resource_id = r.id
WHERE resource_id = $1
  AND starts_at >= $2
  AND ends_at <= $3
  AND status NOT IN ('cancelled', 'no_show')
ORDER BY starts_at;