-- name: AdvanceOnboardingStep :exec
UPDATE onboarding_progress
SET current_step = current_step + 1,
    updated_at   = NOW()
WHERE tenant_id = $1
  AND current_step < $2;