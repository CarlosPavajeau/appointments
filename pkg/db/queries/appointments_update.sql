-- name: UpdateAppointment :exec
UPDATE appointments
SET status        = $1,
    updated_at    = NOW(),
    cancelled_at  = CASE
                        WHEN $1 = 'cancelled'::appointment_status THEN COALESCE(cancelled_at, NOW())
                        WHEN status = 'cancelled'::appointment_status THEN NULL
                        ELSE cancelled_at
        END,
    cancelled_by  = $2,
    cancel_reason = $3,
    completed_at  = $4
WHERE id = $5;