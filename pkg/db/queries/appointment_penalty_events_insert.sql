-- name: InsertAppointmentPenaltyEvent :execrows
INSERT INTO appointment_penalty_events(
    id,
    appointment_id,
    tenant_id,
    customer_id,
    event_type,
    occurred_at,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    NOW()
)
ON CONFLICT (appointment_id, event_type) DO NOTHING;
