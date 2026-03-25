-- name: InsertTenantWhatsappConfig :exec
INSERT INTO tenant_whatsapp_configs(
    id,
    tenant_id,
    activation_status,
    activation_requested_at,
    activation_contact_email,
    activation_notes
) VALUES ($1, $2, 'pending', NOW(), $3, $4)
ON CONFLICT (tenant_id) DO UPDATE
    SET activation_status        = 'pending',
        activation_requested_at  = NOW(),
        activation_contact_email = EXCLUDED.activation_contact_email,
        activation_notes         = EXCLUDED.activation_notes;
