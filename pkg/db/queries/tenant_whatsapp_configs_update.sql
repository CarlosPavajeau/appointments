-- name: UpdateTenantWhatsappConfig :exec
UPDATE tenant_whatsapp_configs
SET access_token     = $1,
    token_expires_at = $2,
    verified_at      = $3,
    updated_at       = NOW()
WHERE id = $4;