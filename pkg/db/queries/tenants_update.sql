-- name: UpdateTenant :exec
UPDATE tenants
SET name       = $1,
    timezone   = $2,
    settings   = $3,
    updated_at = NOW()
WHERE id = $4;