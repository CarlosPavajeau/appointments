-- name: FindTenantByID :one
SELECT id,
       name,
       slug,
       timezone,
       currency,
       plan,
       plan_expires_at,
       appointments_this_month,
       month_reset_at,
       is_active,
       settings,
       created_at,
       updated_at
FROM tenants
WHERE id = $1
  AND is_active = true
LIMIT 1;
