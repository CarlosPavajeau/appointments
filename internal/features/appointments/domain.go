package appointments

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidTransition = errors.New("invalid_status_transition")

type Appointment struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ResourceID     uuid.UUID
	ServiceID      uuid.UUID
	CustomerID     uuid.UUID
	StartsAt       time.Time
	EndsAt         time.Time
	Status         string
	PriceAtBooking float64

	CancelledBy  *string
	CancelReason *string
	CancelledAt  *time.Time

	NoShowReason *string
	NoShowAt     *time.Time

	CheckedInAt *time.Time
	CompletedAt *time.Time
}

type AppointmentWithDetails struct {
	ID             uuid.UUID
	StartsAt       time.Time
	EndsAt         time.Time
	Status         string
	PriceAtBooking float64
	ResourceName   string
	ServiceName    string
	CustomerName   string
}

type ListFilters struct {
	ResourceIDs []uuid.UUID
	ServiceIDs  []uuid.UUID
	CustomerID  *uuid.UUID
}

var validTransitions = map[string][]string{
	"pending":     {"confirmed", "cancelled"},
	"confirmed":   {"in_progress", "cancelled", "no_show"},
	"in_progress": {"completed", "cancelled"},
	"completed":   {},
	"cancelled":   {},
	"no_show":     {},
}
