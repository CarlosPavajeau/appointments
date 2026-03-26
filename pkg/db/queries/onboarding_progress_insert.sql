-- name: InsertOnboardingProgress :exec
INSERT INTO onboarding_progress (id, tenant_id, current_step)
VALUES ($1, $2, $3);