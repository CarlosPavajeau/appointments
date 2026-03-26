-- name: FindResourceById :one
SELECT id,
       tenant_id,
       name,
       type,
       COALESCE(avatar_url, '') as avatar_url,
       is_active,
       sort_order,
       created_at
FROM resources
WHERE id = $1
  AND is_active = true
LIMIT 1;