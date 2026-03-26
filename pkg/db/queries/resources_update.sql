-- name: UpdateResource :exec
UPDATE resources
SET name       = $1,
    type       = $2,
    avatar_url = $3,
    sort_order = $4
WHERE id = $5
  AND tenant_id = $6;