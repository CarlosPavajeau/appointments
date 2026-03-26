-- name: InsertResource :exec
INSERT INTO resources(
    id,
    tenant_id,
    name,
    type,
    avatar_url,
    is_active,
    sort_order
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    true,
    $6
);