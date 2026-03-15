package scheduling

import (
	"context"
	"errors"
	"fmt"
	"time"
	"wappiz/internal/features/appointments"
	"wappiz/internal/features/customers"
	"wappiz/internal/features/resources"
	"wappiz/internal/features/services"

	"wappiz/internal/features/tenants"
	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
)

// AppointmentService defines the appointment operations needed by scheduling.
type AppointmentService interface {
	Create(ctx context.Context, input *appointments.CreateAppointmentInput) (*appointments.Appointment, error)
	GetByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]appointments.Appointment, error)
	GetByID(ctx context.Context, id, tenantID uuid.UUID) (*appointments.Appointment, error)
	Cancel(ctx context.Context, id uuid.UUID, cancelledBy, reason string) error
	GetUpcomingForReminders(ctx context.Context) ([]appointments.Appointment, error)
	MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error
}

const (
	maxDateAttempts = 3
	sessionTTL      = 30 * time.Minute
	freePlanLimit   = 30
)

type UseCases struct {
	sessions       SessionRepository
	appointmentSvc AppointmentService
	services       services.Repository
	resources      resources.Repository
	customers      customers.Repository
	slotFinder     *SlotFinder
	tenantRepo     tenants.Repository
}

func NewUseCases(
	sessions SessionRepository,
	appointmentSvc AppointmentService,
	services services.Repository,
	resources resources.Repository,
	customers customers.Repository,
	availability AvailabilityRepository,
	tenantRepo tenants.Repository,
) *UseCases {
	return &UseCases{
		sessions:       sessions,
		appointmentSvc: appointmentSvc,
		services:       services,
		resources:      resources,
		customers:      customers,
		slotFinder:     NewSlotFinder(availability),
		tenantRepo:     tenantRepo,
	}
}

func (uc *UseCases) ResolveCustomer(ctx context.Context, tenantID uuid.UUID, phone string) (*customers.Customer, error) {
	return uc.customers.FindOrCreate(ctx, tenantID, phone)
}

func (uc *UseCases) CreateSession(ctx context.Context, tenantID, whatsappConfigID, customerID uuid.UUID) (*Session, error) {
	s := &Session{
		ID:               uuid.New(),
		TenantID:         tenantID,
		WhatsappConfigID: whatsappConfigID,
		CustomerID:       customerID,
		Step:             StepSelectService,
		Data:             SessionData{},
		ExpiresAt:        time.Now().Add(sessionTTL),
	}
	return s, uc.sessions.Create(ctx, s)
}

func (uc *UseCases) AdvanceSession(ctx context.Context, s *Session) error {
	s.ExpiresAt = time.Now().Add(sessionTTL)
	return uc.sessions.Update(ctx, s)
}

func (uc *UseCases) GetServices(ctx context.Context, tenantID uuid.UUID) ([]services.Service, error) {
	return uc.services.FindByTenantWithAssignedResource(ctx, tenantID)
}

func (uc *UseCases) ValidateService(ctx context.Context, tenantID uuid.UUID, interactiveID *string) (*services.Service, error) {
	if interactiveID == nil {
		return nil, apperrors.ErrInvalidFormat
	}
	id, err := uuid.Parse(*interactiveID)
	if err != nil {
		return nil, apperrors.ErrInvalidFormat
	}
	svc, err := uc.services.FindByID(ctx, id)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}
	if svc.TenantID != tenantID {
		return nil, apperrors.ErrNotFound
	}
	return svc, nil
}

func (uc *UseCases) GetResourcesForService(ctx context.Context, tenantID, serviceID uuid.UUID) ([]resources.Resource, error) {
	return uc.resources.FindByTenantAndService(ctx, tenantID, serviceID)
}

type DateValidationResult struct {
	StartsAt  time.Time
	Slots     []TimeSlot // empty if is available
	SlotTaken bool
}

func (uc *UseCases) ValidateAndFindSlots(
	ctx context.Context,
	input string,
	tenantTZ string,
	session *Session,
) (*DateValidationResult, error) {

	loc, _ := time.LoadLocation(tenantTZ)

	t, err := ParseDateTime(input, loc)
	if err != nil {
		return nil, apperrors.ErrInvalidFormat
	}

	if t.Before(time.Now()) {
		return nil, apperrors.ErrDateInPast
	}

	service, err := uc.services.FindByID(ctx, *session.Data.ServiceID)
	if err != nil {
		return nil, err
	}

	if session.Data.ResourceID != nil {
		return uc.validateForResource(ctx, t, *session.Data.ResourceID, service)
	}

	return uc.validateForAnyResource(ctx, t, session.TenantID, *session.Data.ServiceID, service)
}

func (uc *UseCases) validateForResource(ctx context.Context, t time.Time, resourceID uuid.UUID, service *services.Service) (*DateValidationResult, error) {
	slots, err := uc.slotFinder.GetAvailableSlots(ctx, resourceID, t, service)
	if err != nil {
		return nil, err
	}

	if len(slots) == 0 {
		return nil, apperrors.ErrDayOff
	}

	for _, slot := range slots {
		if slot.StartsAt.Equal(t) {
			return &DateValidationResult{StartsAt: t}, nil
		}
	}

	suggestions, err := uc.slotFinder.GetSuggestedSlots(ctx, resourceID, t, service)
	if err != nil {
		return nil, err
	}

	return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: suggestions}, nil
}

func (uc *UseCases) validateForAnyResource(ctx context.Context, t time.Time, tenantID, serviceID uuid.UUID, service *services.Service) (*DateValidationResult, error) {
	resources, err := uc.resources.FindByTenantAndService(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}

	for _, res := range resources {
		slots, err := uc.slotFinder.GetAvailableSlots(ctx, res.ID, t, service)
		if err != nil {
			continue
		}
		for _, slot := range slots {
			if slot.StartsAt.Equal(t) {
				result := &DateValidationResult{StartsAt: t}
				// Assign the resource that has the slot available
				return result, nil
			}
		}
	}

	// Find suggestions across all resources
	var allSuggestions []TimeSlot
	for _, res := range resources {
		suggestions, _ := uc.slotFinder.GetSuggestedSlots(ctx, res.ID, t, service)
		allSuggestions = append(allSuggestions, suggestions...)
		if len(allSuggestions) >= 3 {
			break
		}
	}

	return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: allSuggestions}, nil
}

func (uc *UseCases) CreateAppointment(ctx context.Context, session *Session, tenantTZ string) (*appointments.Appointment, error) {
	tenant, err := uc.tenantRepo.FindByID(ctx, session.TenantID)
	if err != nil {
		return nil, err
	}

	if tenant.Plan == "free" && tenant.AppointmentsThisMonth >= freePlanLimit {
		return nil, apperrors.ErrPlanLimitReached
	}

	service, err := uc.services.FindByID(ctx, *session.Data.ServiceID)
	if err != nil {
		return nil, err
	}

	startsAt := *session.Data.StartsAt
	endsAt := startsAt.Add(time.Duration(service.DurationMinutes) * time.Minute)

	a, err := uc.appointmentSvc.Create(ctx, &appointments.CreateAppointmentInput{
		TenantID:       session.TenantID,
		ResourceID:     *session.Data.ResourceID,
		ServiceID:      *session.Data.ServiceID,
		CustomerID:     session.CustomerID,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Status:         "confirmed",
		PriceAtBooking: service.Price,
	})
	if err != nil {
		if isOverlapError(err) {
			return nil, apperrors.ErrOverlap
		}
		return nil, err
	}

	uc.tenantRepo.IncrementAppointmentCount(ctx, session.TenantID)

	return a, nil
}

func isOverlapError(err error) bool {
	return err != nil && (contains(err.Error(), "no_overlap") ||
		contains(err.Error(), "exclusion constraint"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 &&
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}

func BuildErrorMessage(err error, input string, suggestions []TimeSlot) string {
	switch {
	case errors.Is(err, apperrors.ErrInvalidFormat):
		return fmt.Sprintf(
			"No pude entender *%s* como una fecha válida 😅\n\n"+
				"Usa este formato:\n*DD/MM HH:mm AM/PM*\n\nEjemplo: *02/03 09:00 AM*", input)
	case errors.Is(err, apperrors.ErrDateInPast):
		return "Esa fecha ya pasó 📅 Por favor elige una fecha futura."
	case errors.Is(err, apperrors.ErrDayOff):
		return "Ese día no atendemos. Trabajamos de *lunes a sábado*.\nPor favor elige otro día."
	case errors.Is(err, apperrors.ErrOutsideHours):
		return "Ese horario está fuera de nuestro horario de atención (*9:00 AM – 7:00 PM*)."
	case errors.Is(err, apperrors.ErrPlanLimitReached):
		return "Lo sentimos, esta barbería ha alcanzado su límite de citas del mes 😔"
	case errors.Is(err, apperrors.ErrOverlap):
		if len(suggestions) == 0 {
			return "Ese horario ya no está disponible 😔 Por favor intenta con otra fecha."
		}
		msg := "Ese horario acaba de ser tomado 😔 Estas son las opciones más cercanas:\n\n"
		for _, s := range suggestions {
			msg += fmt.Sprintf("• %s\n", s.StartsAt.Format("02/01 03:04 PM"))
		}
		return msg
	}
	return "Ocurrió un error inesperado. Por favor intenta de nuevo."
}
