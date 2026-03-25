-- name: InsertTenant :exec
INSERT INTO tenants(
    id,
    name,
    slug,
    timezone,
    currency,
    plan,
    appointments_this_month,
    month_reset_at,
    is_active,
    settings
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    0,
    $7,
    true,
    $8
);