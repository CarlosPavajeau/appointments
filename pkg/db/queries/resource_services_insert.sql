-- name: InsertResourceService :exec
INSERT INTO resource_services (resource_id, service_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;