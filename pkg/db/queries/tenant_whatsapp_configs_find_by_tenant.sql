-- name: FindTenantWhatsappConfig :one
SELECT id,
       tenant_id,
       waba_id,
       phone_number_id,
       display_phone_number,
       access_token,
       token_expires_at,
       is_active,
       verified_at,
       created_at,
       updated_at
FROM tenant_whatsapp_configs
WHERE tenant_id = $1
  AND is_active = true
LIMIT 1;