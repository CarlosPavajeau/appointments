-- name: FindCustomerPenaltyCounts :one
SELECT no_show_count,
       late_cancel_count
FROM customers
WHERE id = $1
  AND tenant_id = $2;
