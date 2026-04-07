-- name: MarkAppointmentReminderSentByType :exec
UPDATE appointments
SET reminder_24h_sent_at = CASE
                               WHEN sqlc.arg(reminder_type)::text = '24h'
                                   THEN COALESCE(reminder_24h_sent_at, NOW())
                               ELSE reminder_24h_sent_at
    END,
    reminder_1h_sent_at  = CASE
                               WHEN sqlc.arg(reminder_type)::text = '1h'
                                   THEN COALESCE(reminder_1h_sent_at, NOW())
                               ELSE reminder_1h_sent_at
        END
WHERE id = sqlc.arg(appointment_id)::uuid
  AND status = 'confirmed';
