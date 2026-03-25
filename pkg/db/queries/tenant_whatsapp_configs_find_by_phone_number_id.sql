-- name: FindTenantWhatsappConfigByPhoneNumberID :one
SELECT twc.id,
       twc.tenant_id,
       twc.waba_id,
       twc.phone_number_id,
       twc.display_phone_number,
       twc.access_token,
       twc.token_expires_at,
       twc.is_active,
       twc.verified_at,
       twc.created_at,
       twc.updated_at,
       t.name      AS tenant_name,
       t.slug      AS tenant_slug,
       t.timezone  AS tenant_timezone,
       t.currency  AS tenant_currency,
       t.plan      AS tenant_plan,
       t.settings  AS tenant_settings,
       t.is_active AS tenant_active,
       t.month_reset_at,
       t.appointments_this_month
FROM tenant_whatsapp_configs twc
         JOIN tenants t ON t.id = twc.tenant_id
WHERE twc.phone_number_id = $1
  AND twc.is_active = true
  AND t.is_active = true
LIMIT 1;