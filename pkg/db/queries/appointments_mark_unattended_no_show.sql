-- name: MarkUnattendedAppointmentsNoShow :many
UPDATE appointments
SET status = 'no_show',
    updated_at = NOW(),
    cancelled_by = NULL,
    cancel_reason = NULL,
    cancelled_at = NULL,
    completed_at = NULL
WHERE status = 'confirmed'
  AND starts_at <= NOW() - INTERVAL '30 minutes'
RETURNING id, tenant_id, customer_id;
