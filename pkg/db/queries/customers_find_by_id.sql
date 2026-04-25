-- name: FindCustomerByID :one
SELECT id,
       tenant_id,
       phone_number,
       name,
       is_blocked,
       no_show_count,
       late_cancel_count,
       created_at
FROM customers
WHERE id = $1
LIMIT 1;