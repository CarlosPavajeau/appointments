package appointments

import (
	"context"
	"fmt"
	"log"
	"time"
	"wappiz/internal/features/customers"
	"wappiz/internal/features/tenants"
	"wappiz/internal/platform/whatsapp"

	"github.com/google/uuid"
)

type UseCases struct {
	repository   Repository
	customerRepo customers.Repository
	tenantRepo   tenants.Repository
	wa           whatsapp.Client
}

func NewUseCases(repository Repository, customerRepo customers.Repository, tenantRepo tenants.Repository, wa whatsapp.Client) *UseCases {
	return &UseCases{
		repository:   repository,
		customerRepo: customerRepo,
		tenantRepo:   tenantRepo,
		wa:           wa,
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

func (uc *UseCases) UpdateStatus(ctx context.Context, id, tenantID uuid.UUID, newStatus string, updatedBy *string, updatedByRole, reason string) error {
	appt, err := uc.repository.FindByID(ctx, id, tenantID)
	if err != nil {
		return err
	}

	allowed := validTransitions[appt.Status]
	for _, s := range allowed {
		if s == newStatus {
			if err := uc.repository.UpdateStatusWithHistory(ctx, id, newStatus, updatedBy, reason, &AppointmentStatusHistory{
				ID:            uuid.New(),
				AppointmentID: id,
				FromStatus:    appt.Status,
				ToStatus:      newStatus,
				ChangedBy:     updatedBy,
				ChangedByRole: updatedByRole,
				Reason:        reason,
			}); err != nil {
				return err
			}

			if newStatus == "cancelled" && updatedByRole != "customer" {
				uc.sendCancellationNotification(ctx, appt)
			}

			return nil
		}
	}

	return ErrInvalidTransition
}

func (uc *UseCases) sendCancellationNotification(ctx context.Context, appt *Appointment) {
	customer, err := uc.customerRepo.FindByID(ctx, appt.CustomerID)
	if err != nil {
		log.Printf("[appointments] sendCancellationNotification: failed to find customer %s: %v", appt.CustomerID, err)
		return
	}

	tenant, err := uc.tenantRepo.FindByID(ctx, appt.TenantID)
	if err != nil {
		log.Printf("[appointments] sendCancellationNotification: failed to find tenant %s: %v", appt.TenantID, err)
		return
	}

	waConfig, err := uc.tenantRepo.FindWhatsappConfig(ctx, appt.TenantID)
	if err != nil {
		log.Printf("[appointments] sendCancellationNotification: failed to find whatsapp config for tenant %s: %v", appt.TenantID, err)
		return
	}

	body := fmt.Sprintf("Tu cita del *%s* ha sido cancelada.\n\n%s",
		appt.StartsAt.Format("02/01/2006 03:04 PM"),
		tenant.CancellationMessage(),
	)

	if err := uc.wa.SendText(ctx, customer.PhoneNumber, waConfig.PhoneNumberID, waConfig.AccessToken, body); err != nil {
		log.Printf("[appointments] sendCancellationNotification: failed to send whatsapp to %s: %v", customer.PhoneNumber, err)
	}
}
