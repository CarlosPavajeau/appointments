-- name: DeleteResourceService :exec
DELETE
FROM resource_services
WHERE resource_id = $1;