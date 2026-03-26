-- name: DeleteResource :exec
UPDATE resources
SET is_active = false
WHERE id = $1
  AND tenant_id = $2;