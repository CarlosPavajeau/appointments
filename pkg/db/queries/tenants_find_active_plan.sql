-- name: FindActivePlanByTenant :one
SELECT p.id,
       p.external_id,
       p.environment,
       p.features
FROM subscriptions ts
         JOIN plans p ON p.id = ts.plan_id
WHERE ts.tenant_id = $1
  AND ts.status = 'active'
  AND ts.environment = $2
LIMIT 1;