-- name: FindPendingAppointmentReminderEvents :many
SELECT e.id,
       e.appointment_id,
       e.tenant_id,
       e.customer_id,
       e.reminder_type,
       e.attempts,
       a.starts_at
FROM appointment_reminder_events e
         JOIN appointments a ON a.id = e.appointment_id
WHERE e.sent_at IS NULL
  AND e.attempts < 5
  AND a.status = 'confirmed'
ORDER BY e.created_at
LIMIT 500;
