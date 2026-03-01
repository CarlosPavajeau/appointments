package scheduling

import (
	"time"

	"github.com/google/uuid"
)

type SessionStep string

const (
	StepSelectService  SessionStep = "SELECT_SERVICE"
	StepSelectResource SessionStep = "SELECT_RESOURCE"
	StepSelectDate     SessionStep = "SELECT_DATE"
	StepSelectTime     SessionStep = "SELECT_TIME"
	StepAwaitingName   SessionStep = "AWAITING_NAME"
	StepConfirm        SessionStep = "CONFIRM"
)

type SessionData struct {
	ServiceID     *uuid.UUID `json:"service_id,omitempty"`
	ResourceID    *uuid.UUID `json:"resource_id,omitempty"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	DateAttempts  int        `json:"date_attempts"`
	ConfirmedName *string    `json:"confirmed_name,omitempty"`
}

type Session struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	WhatsappConfigID uuid.UUID
	CustomerID       uuid.UUID
	Step             SessionStep
	Data             SessionData
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type IncomingMessage struct {
	TenantID         uuid.UUID
	WhatsappConfigID uuid.UUID
	PhoneNumberID    string
	AccessToken      string
	From             string
	Body             string
	InteractiveID    *string
	ReceivedAt       time.Time
}

type Service struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Name            string
	Description     string
	DurationMinutes int
	BufferMinutes   int
	Price           float64
	SortOrder       int
}

type Resource struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Type      string
	SortOrder int
}

type TimeSlot struct {
	StartsAt     time.Time
	EndsAt       time.Time
	ResourceID   uuid.UUID
	ResourceName string
}

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

type WorkingHours struct {
	DayOfWeek int
	StartTime string // "09:00"
	EndTime   string // "19:00"
}

type ScheduleOverride struct {
	Date      time.Time
	IsDayOff  bool
	StartTime *string
	EndTime   *string
}
