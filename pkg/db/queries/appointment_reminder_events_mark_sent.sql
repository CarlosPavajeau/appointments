-- name: MarkAppointmentReminderEventSent :exec
UPDATE appointment_reminder_events
SET sent_at = NOW(),
    last_attempt_at = NOW(),
    attempts = attempts + 1,
    last_error = NULL
WHERE id = $1
  AND sent_at IS NULL;
