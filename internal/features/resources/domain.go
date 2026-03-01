package resources

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ResourceType string

const (
	ResourceTypeBarber ResourceType = "barber"
	ResourceTypeChair  ResourceType = "chair"
)

type Resource struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Name         string
	Type         ResourceType
	AvatarURL    string
	IsActive     bool
	SortOrder    int
	WorkingHours []WorkingHours
	CreatedAt    time.Time
}

type WorkingHours struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	DayOfWeek  int    // 0=Sunday, 1=Monday ... 6=Saturday
	StartTime  string // "09:00"
	EndTime    string // "19:00"
	IsActive   bool
}

type ScheduleOverride struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	Date       time.Time
	IsDayOff   bool
	StartTime  *string // nil if IsDayOff = true
	EndTime    *string // nil if IsDayOff = true
	Reason     string
	CreatedAt  time.Time
}

func (wh *WorkingHours) DayName() string {
	days := []string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}
	if wh.DayOfWeek < 0 || wh.DayOfWeek > 6 {
		return "Desconocido"
	}
	return days[wh.DayOfWeek]
}

func (r *Resource) Validate() error {
	if r.Name == "" {
		return ErrNameRequired
	}
	if r.Type != ResourceTypeBarber && r.Type != ResourceTypeChair {
		return ErrInvalidType
	}
	return nil
}

func (wh *WorkingHours) Validate() error {
	if wh.DayOfWeek < 0 || wh.DayOfWeek > 6 {
		return ErrInvalidDayOfWeek
	}
	start, err := parseTime(wh.StartTime)
	if err != nil {
		return ErrInvalidTime
	}
	end, err := parseTime(wh.EndTime)
	if err != nil {
		return ErrInvalidTime
	}
	if !start.Before(end) {
		return ErrStartAfterEnd
	}
	return nil
}

func (so *ScheduleOverride) Validate() error {
	if so.Date.IsZero() {
		return ErrInvalidDate
	}
	if so.Date.Before(time.Now().Truncate(24 * time.Hour)) {
		return ErrDateInPast
	}
	if !so.IsDayOff {
		if so.StartTime == nil || so.EndTime == nil {
			return ErrTimeRequired
		}
		start, err := parseTime(*so.StartTime)
		if err != nil {
			return ErrInvalidTime
		}
		end, err := parseTime(*so.EndTime)
		if err != nil {
			return ErrInvalidTime
		}
		if !start.Before(end) {
			return ErrStartAfterEnd
		}
	}
	return nil
}

func parseTime(t string) (time.Time, error) {
	parsed, err := time.Parse("15:04", t)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %s", t)
	}
	return parsed, nil
}

var (
	ErrNameRequired     = resourceError("name is required")
	ErrInvalidType      = resourceError("type must be 'barber' or 'chair'")
	ErrInvalidDayOfWeek = resourceError("day_of_week must be between 0 and 6")
	ErrInvalidTime      = resourceError("time must be in HH:MM format")
	ErrStartAfterEnd    = resourceError("start_time must be before end_time")
	ErrInvalidDate      = resourceError("date is required")
	ErrDateInPast       = resourceError("date cannot be in the past")
	ErrTimeRequired     = resourceError("start_time and end_time are required when is_day_off is false")
)

type resourceError string

func (e resourceError) Error() string { return string(e) }
