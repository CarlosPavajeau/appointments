-- name: FindOnboardingProgressByTenant :one
SELECT id, tenant_id, current_step, completed_at, created_at, updated_at
FROM onboarding_progress
WHERE tenant_id = $1
LIMIT 1;