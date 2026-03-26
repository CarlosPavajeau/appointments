-- name: CompleteOnboardingProgress :exec
UPDATE onboarding_progress
SET completed_at = NOW(),
    updated_at   = NOW()
WHERE tenant_id = $1;