-- name: InsertScheduleOverride :exec
INSERT INTO schedule_overrides(
    id,
    resource_id,
    date,
    is_day_off,
    start_time,
    end_time,
    reason
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
) ON CONFLICT (resource_id, date) DO UPDATE
    SET is_day_off = EXCLUDED.is_day_off,
        start_time = EXCLUDED.start_time,
        end_time   = EXCLUDED.end_time,
        reason     = EXCLUDED.reason;