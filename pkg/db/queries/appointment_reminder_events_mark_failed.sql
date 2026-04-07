-- name: MarkAppointmentReminderEventFailed :exec
UPDATE appointment_reminder_events
SET last_attempt_at = NOW(),
    attempts = attempts + 1,
    last_error = $2
WHERE id = $1
  AND sent_at IS NULL;
