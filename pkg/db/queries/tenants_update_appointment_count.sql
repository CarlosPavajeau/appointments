-- name: UpdateTenantAppointmentCount :exec
UPDATE tenants
SET appointments_this_month = $2,
    updated_at              = NOW()
WHERE id = $1;