package scheduling

import (
	"appointments/internal/features/customers"
	"appointments/internal/features/resources"
	"appointments/internal/features/services"
	"appointments/internal/features/tenants"
	"context"
	"fmt"
	"log"
	"time"

	"appointments/internal/platform/whatsapp"
)

type ReminderJob struct {
	appointments AppointmentRepository
	services     services.Repository
	resources    resources.Repository
	customers    customers.Repository
	tenantRepo   tenants.Repository
	wa           whatsapp.Client
}

func NewReminderJob(
	appointments AppointmentRepository,
	services services.Repository,
	resources resources.Repository,
	clients customers.Repository,
	tenantRepo tenants.Repository,
	wa whatsapp.Client,
) *ReminderJob {
	return &ReminderJob{
		appointments: appointments,
		services:     services,
		resources:    resources,
		customers:    clients,
		tenantRepo:   tenantRepo,
		wa:           wa,
	}
}

func (j *ReminderJob) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("reminder job started")

	for {
		select {
		case <-ctx.Done():
			log.Println("reminder job stopped")
			return
		case <-ticker.C:
			if err := j.process(ctx); err != nil {
				log.Printf("reminder job error: %v", err)
			}
		}
	}
}

func (j *ReminderJob) process(ctx context.Context) error {
	upcoming, err := j.appointments.FindUpcomingForReminders(ctx)
	if err != nil {
		return err
	}

	for _, a := range upcoming {
		if err := j.sendReminder(ctx, a); err != nil {
			log.Printf("failed to send reminder for appointment %s: %v", a.ID, err)
		}
	}
	return nil
}

func (j *ReminderJob) sendReminder(ctx context.Context, a Appointment) error {
	client, err := j.customers.FindByID(ctx, a.CustomerID)
	if err != nil {
		return err
	}

	svc, err := j.services.FindByID(ctx, a.ServiceID)
	if err != nil {
		return err
	}

	res, err := j.resources.FindByID(ctx, a.ResourceID)
	if err != nil {
		return err
	}

	tenant, err := j.tenantRepo.FindByID(ctx, a.TenantID)
	if err != nil {
		return err
	}

	waConfig, err := j.tenantRepo.FindWhatsappConfig(ctx, a.TenantID)
	if err != nil {
		return err
	}

	timeUntil := time.Until(a.StartsAt)
	reminderType := "24h"
	timeLabel := "mañana"
	if timeUntil < 2*time.Hour {
		reminderType = "1h"
		timeLabel = "en 1 hora"
	}

	clientName := "Cliente"
	if client.Name != nil {
		clientName = *client.Name
	}

	body := fmt.Sprintf(
		"⏰ *Recordatorio de cita*\n\n"+
			"Hola %s, te recordamos que tienes una cita *%s*:\n\n"+
			"✂️ %s con %s\n"+
			"📅 %s\n"+
			"📍 %s\n\n"+
			"Si necesitas cancelar escríbenos aquí.",
		clientName,
		timeLabel,
		svc.Name, res.Name,
		a.StartsAt.Format("02/01/2006 03:04 PM"),
		tenant.Name,
	)

	if err := j.wa.SendText(ctx, client.PhoneNumber, waConfig.PhoneNumberID, waConfig.AccessToken, body); err != nil {
		return err
	}

	return j.appointments.MarkReminderSent(ctx, a.ID, reminderType)
}
