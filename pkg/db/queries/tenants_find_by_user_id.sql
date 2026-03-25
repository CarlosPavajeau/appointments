-- name: FindTenantByUserId :one
SELECT t.id,
       t.name,
       t.slug,
       t.timezone,
       t.currency,
       t.plan,
       t.plan_expires_at,
       t.appointments_this_month,
       t.month_reset_at,
       t.is_active,
       t.settings,
       t.created_at,
       t.updated_at
FROM tenants t
         JOIN tenant_users tu ON tu.tenant_id = t.id
WHERE tu.user_id = $1
  AND t.is_active = true
LIMIT 1;