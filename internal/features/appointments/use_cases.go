package appointments

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UseCases struct {
	repository Repository
}

func NewUseCases(repository Repository) *UseCases {
	return &UseCases{
		repository: repository,
	}
}

type CreateAppointmentInput struct {
	TenantID       uuid.UUID
	ResourceID     uuid.UUID
	ServiceID      uuid.UUID
	CustomerID     uuid.UUID
	StartsAt       time.Time
	EndsAt         time.Time
	Status         string
	PriceAtBooking float64
}

func (uc *UseCases) Create(ctx context.Context, input *CreateAppointmentInput) (*Appointment, error) {
	appointment := &Appointment{
		ID:             uuid.New(),
		TenantID:       input.TenantID,
		ResourceID:     input.ResourceID,
		ServiceID:      input.ServiceID,
		CustomerID:     input.CustomerID,
		StartsAt:       input.StartsAt,
		EndsAt:         input.EndsAt,
		Status:         input.Status,
		PriceAtBooking: input.PriceAtBooking,
	}

	if err := uc.repository.Save(ctx, appointment); err != nil {
		return nil, err
	}

	return appointment, nil
}

func (uc *UseCases) GetByCustomer(ctx context.Context, tenantID, customerID uuid.UUID) ([]Appointment, error) {
	return uc.repository.FindByCustomerID(ctx, tenantID, customerID)
}

func (uc *UseCases) GetByCustomerWithDetails(ctx context.Context, tenantID, customerID uuid.UUID) ([]AppointmentWithDetails, error) {
	return uc.repository.FindByCustomerIDWithDetails(ctx, tenantID, customerID)
}

func (uc *UseCases) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Appointment, error) {
	return uc.repository.FindByID(ctx, id, tenantID)
}

func (uc *UseCases) Cancel(ctx context.Context, id uuid.UUID, cancelledBy, reason string) error {
	return uc.repository.UpdateStatus(ctx, id, "cancelled", cancelledBy, reason)
}

func (uc *UseCases) GetUpcomingForReminders(ctx context.Context) ([]Appointment, error) {
	return uc.repository.FindUpcomingForReminders(ctx)
}

func (uc *UseCases) MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error {
	return uc.repository.MarkReminderSent(ctx, id, reminderType)
}

func (uc *UseCases) GetByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]Appointment, error) {
	return uc.repository.FindByDate(ctx, tenantID, date)
}

func (uc *UseCases) Search(ctx context.Context, tenantID uuid.UUID, date time.Time, filters ListFilters) ([]AppointmentWithDetails, error) {
	return uc.repository.Search(ctx, tenantID, date, filters)
}

func (uc *UseCases) GetStatusHistory(ctx context.Context, id, tenantID uuid.UUID) ([]AppointmentStatusHistory, error) {
	if _, err := uc.repository.FindByID(ctx, id, tenantID); err != nil {
		return nil, err
	}
	return uc.repository.FindStatusHistory(ctx, id, tenantID)
}

func (uc *UseCases) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, newStatus, updatedBy, updatedByRole, reason string) error {
	appt, err := uc.repository.FindByID(ctx, id, tenantID)
	if err != nil {
		return err
	}

	allowed := validTransitions[appt.Status]
	for _, s := range allowed {
		if s == newStatus {
			return uc.repository.UpdateStatusWithHistory(ctx, id, newStatus, updatedBy, reason, &AppointmentStatusHistory{
				ID:            uuid.New(),
				AppointmentID: id,
				FromStatus:    appt.Status,
				ToStatus:      newStatus,
				ChangedBy:     updatedBy,
				ChangedByRole: updatedByRole,
				Reason:        reason,
			})
		}
	}

	return ErrInvalidTransition
}
