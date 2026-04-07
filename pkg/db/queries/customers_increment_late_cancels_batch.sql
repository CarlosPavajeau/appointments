-- name: IncrementCustomersLateCancelsBatch :exec
WITH args AS (
    SELECT
        sqlc.arg(customer_ids)::uuid[] AS customer_ids,
        sqlc.arg(tenant_ids)::uuid[] AS tenant_ids,
        sqlc.arg(increments)::int[] AS increments
),
updates AS (
    SELECT
        UNNEST(args.customer_ids) AS customer_id,
        UNNEST(args.tenant_ids) AS tenant_id,
        UNNEST(args.increments) AS increment_by
    FROM args
)
UPDATE customers AS c
SET late_cancel_count = c.late_cancel_count + updates.increment_by
FROM updates
WHERE c.id = updates.customer_id
  AND c.tenant_id = updates.tenant_id;
