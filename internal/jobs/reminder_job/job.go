package reminder_job

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"wappiz/pkg/date_formatter"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
	"wappiz/pkg/whatsapp"

	"github.com/google/uuid"
)

type job struct {
	db       db.Database
	whatsapp whatsapp.Client
}

func New(cfg Config) *job {
	return &job{
		db:       cfg.DB,
		whatsapp: cfg.Whatsapp,
	}
}

func (j *job) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger.Info("[reminder_job] started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("[reminder_job] stopped")
			return
		case <-ticker.C:
			if err := j.process(ctx); err != nil {
				logger.Error("[reminder_job] failed to process job",
					"err", err)
			}
		}
	}
}

func (j *job) process(ctx context.Context) error {
	upcoming, err := db.Query.FindUpcomingAppointments(ctx, j.db.Primary())
	if err != nil {
		return err
	}

	waConfigs := make(map[uuid.UUID]db.FindTenantWhatsappConfigRow)
	for _, a := range upcoming {
		waConfig, ok := waConfigs[a.TenantID]

		if !ok {
			waConfig, err = db.Query.FindTenantWhatsappConfig(ctx, j.db.Primary(), a.TenantID)
			if err != nil {
				continue
			}

			waConfigs[a.TenantID] = waConfig
		}

		if err := j.sendReminder(ctx, a, waConfig); err != nil {
			logger.Warn("[reminder_job] failed to send reminder",
				"err", err)
		}
	}

	return nil
}

func (j *job) sendReminder(ctx context.Context, appointment db.FindUpcomingAppointmentsRow, waConfig db.FindTenantWhatsappConfigRow) error {
	if !waConfig.PhoneNumberID.Valid || !waConfig.AccessToken.Valid {
		return nil
	}

	customer, err := db.Query.FindCustomerByID(ctx, j.db.Primary(), appointment.CustomerID)
	if err != nil {
		return err
	}

	timeUntil := time.Until(appointment.StartsAt)
	reminderType := "24h"
	timeLabel := "mañana"

	if timeUntil < 2*time.Hour {
		reminderType = "1h"
		timeLabel = "en 1 hora"
	}

	body := fmt.Sprintf(
		"⏰ *Recordatorio de cita*\n\n"+
			"Hola, te recordamos que tienes una cita *%s*:\n\n"+
			"📅 %s\n"+
			"Si necesitas cancelar escríbenos aquí.",
		timeLabel,
		date_formatter.FormatTime(appointment.StartsAt, "Monday, 02 de January de 2006 a las 3:04 PM"),
	)

	if err := j.whatsapp.SendText(ctx, customer.PhoneNumber,
		waConfig.PhoneNumberID.String, waConfig.AccessToken.String, body,
	); err != nil {
		return err
	}

	var reminder24hSentAt sql.NullTime
	var reminder1hSentAt sql.NullTime

	if reminderType == "24h" {
		reminder24hSentAt = sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		}
	}

	if reminderType == "1h" {
		reminder1hSentAt = sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		}
	}

	return db.Query.Mark24hAppointmentReminderSent(ctx, j.db.Primary(), db.Mark24hAppointmentReminderSentParams{
		Reminder24hSentAt: reminder24hSentAt,
		Reminder1hSentAt:  reminder1hSentAt,
		ID:                appointment.ID,
	})
}
