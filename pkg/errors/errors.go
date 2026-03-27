package errors

import "errors"

var (
	ErrNotFound          = errors.New("not_found")
	ErrSessionNotFound   = errors.New("session_not_found")
	ErrInvalidFormat     = errors.New("invalid_format")
	ErrDateInPast        = errors.New("date_in_past")
	ErrDayOff            = errors.New("day_off")
	ErrOutsideHours      = errors.New("outside_working_hours")
	ErrSlotTaken         = errors.New("slot_taken")
	ErrNoSlotsAvailable  = errors.New("no_slots_available")
	ErrPlanLimitReached  = errors.New("plan_limit_reached")
	ErrClientBlocked     = errors.New("client_blocked")
	ErrOverlap           = errors.New("appointment_overlap")
	ErrEmailAlreadyInUse = errors.New("email_already_in_use")
)
