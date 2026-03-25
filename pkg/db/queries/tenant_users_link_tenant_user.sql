-- name: LinkTenantUser :exec
INSERT INTO tenant_users (user_id, tenant_id, role)
VALUES ($1, $2, 'admin')
ON CONFLICT (user_id, tenant_id) DO NOTHING;