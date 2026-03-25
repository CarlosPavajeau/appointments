-- name: FindTenantPendingActivations :many
SELECT wc.tenant_id,
       t.name                                    AS tenant_name,
       COALESCE(wc.activation_contact_email, '') AS contact_email,
       wc.activation_status,
       wc.activation_requested_at,
       COALESCE(wc.activation_notes, '')         AS activation_notes
FROM tenant_whatsapp_configs wc
         JOIN tenants t ON t.id = wc.tenant_id
WHERE wc.activation_status = 'pending'
ORDER BY wc.activation_requested_at;