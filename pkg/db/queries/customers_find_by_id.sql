-- name: FindCustomerByID :one
SELECT c.id,
       c.tenant_id,
       c.phone_number,
       c.name,
       c.is_blocked,
       c.no_show_count,
       c.late_cancel_count,
       COUNT(a.id) as appointment_count,
       c.created_at
FROM customers c
         INNER JOIN appointments a ON c.id = a.customer_id
WHERE c.id = $1
GROUP BY c.id
LIMIT 1;