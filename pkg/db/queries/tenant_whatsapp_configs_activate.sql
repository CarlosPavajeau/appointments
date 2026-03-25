-- name: ActivateTenantWhatsappConfig :exec
UPDATE tenant_whatsapp_configs
SET waba_id              = $1,
    phone_number_id      = $2,
    display_phone_number = $3,
    access_token         = $4,
    activation_status    = 'active',
    is_active            = true,
    verified_at          = NOW()
WHERE tenant_id = $5;
