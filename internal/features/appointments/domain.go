package appointments

import (
	"time"

	"github.com/google/uuid"
)

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
