-- name: UpsertWorkingHours :exec
INSERT INTO working_hours(
    id,
    resource_id,
    day_of_week,
    start_time,
    end_time,
    is_active
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
) ON CONFLICT (resource_id, day_of_week) DO UPDATE
    SET start_time = EXCLUDED.start_time,
        end_time   = EXCLUDED.end_time,
        is_active  = EXCLUDED.is_active;