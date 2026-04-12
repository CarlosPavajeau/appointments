package reminder_job

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"wappiz/pkg/crypto"
	"wappiz/pkg/date_formatter"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
	"wappiz/pkg/whatsapp"

	"github.com/google/uuid"
)

type job struct {
	db       db.Database
	whatsapp whatsapp.Client
	crypto   *crypto.Service
}

func New(cfg Config) *job {
	return &job{
		db:       cfg.DB,
		whatsapp: cfg.Whatsapp,
		crypto:   cfg.Crypto,
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
	tx, err := j.db.Primary().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.Query.ClaimDueAppointmentReminderEvents(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	pending, err := db.Query.FindPendingAppointmentReminderEvents(ctx, j.db.Primary())
	if err != nil {
		return err
	}

	waConfigs := make(map[uuid.UUID]db.FindTenantWhatsappConfigRow)
	customers := make(map[uuid.UUID]db.FindCustomerByIDRow)

	for _, reminder := range pending {
		waConfig, ok := waConfigs[reminder.TenantID]

		if !ok {
			waConfig, err = db.Query.FindTenantWhatsappConfig(ctx, j.db.Primary(), reminder.TenantID)
			if err != nil {
				j.markReminderFailed(ctx, reminder.ID, err)
				continue
			}

			waConfigs[reminder.TenantID] = waConfig
		}

		customer, ok := customers[reminder.CustomerID]
		if !ok {
			customer, err = db.Query.FindCustomerByID(ctx, j.db.Primary(), reminder.CustomerID)
			if err != nil {
				j.markReminderFailed(ctx, reminder.ID, err)
				continue
			}
			customers[reminder.CustomerID] = customer
		}

		if err := j.sendReminder(ctx, reminder, customer, waConfig); err != nil {
			j.markReminderFailed(ctx, reminder.ID, err)
			logger.Warn("[reminder_job] failed to send reminder",
				"err", err)
			continue
		}

		if err := j.markReminderSent(ctx, reminder); err != nil {
			logger.Warn("[reminder_job] failed to mark reminder as sent",
				"event_id", reminder.ID,
				"err", err)
		}
	}

	return nil
}

func (j *job) sendReminder(
	ctx context.Context,
	reminder db.FindPendingAppointmentReminderEventsRow,
	customer db.FindCustomerByIDRow,
	waConfig db.FindTenantWhatsappConfigRow,
) error {
	if !waConfig.PhoneNumberID.Valid || !waConfig.AccessToken.Valid {
		return nil
	}

	timeLabel := "en 1 hora"
	if reminder.ReminderType == "24h" {
		timeLabel = "mañana"
	}

	// Fallback guard for any legacy rows.
	if reminder.ReminderType != "24h" && reminder.ReminderType != "1h" {
		timeUntil := time.Until(reminder.StartsAt)
		timeLabel = "mañana"
		if timeUntil < 2*time.Hour {
			timeLabel = "en 1 hora"
		}
	}

	body := fmt.Sprintf(
		"⏰ *Recordatorio de cita*\n\n"+
			"Hola, te recordamos que tienes una cita *%s*:\n\n"+
			"📅 %s\n"+
			"Si necesitas cancelar escríbenos aquí.",
		timeLabel,
		date_formatter.FormatTime(reminder.StartsAt, "Monday, 02 de January de 2006 a las 3:04 PM"),
	)

	decrypted, err := j.crypto.Decrypt(waConfig.AccessToken.String)
	if err != nil {
		return err
	}

	if err := j.whatsapp.SendText(ctx, customer.PhoneNumber, waConfig.PhoneNumberID.String, decrypted, body); err != nil {
		return err
	}

	return nil
}

func (j *job) markReminderSent(ctx context.Context, reminder db.FindPendingAppointmentReminderEventsRow) error {
	tx, err := j.db.Primary().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := db.Query.MarkAppointmentReminderSentByType(ctx, tx, db.MarkAppointmentReminderSentByTypeParams{
		ReminderType:  reminder.ReminderType,
		AppointmentID: reminder.AppointmentID,
	}); err != nil {
		return err
	}

	if err := db.Query.MarkAppointmentReminderEventSent(ctx, tx, reminder.ID); err != nil {
		return err
	}

	return tx.Commit()
}

func (j *job) markReminderFailed(ctx context.Context, eventID uuid.UUID, reminderErr error) {
	errMsg := reminderErr.Error()
	if len(errMsg) > 1000 {
		errMsg = errMsg[:1000]
	}

	if err := db.Query.MarkAppointmentReminderEventFailed(ctx, j.db.Primary(), db.MarkAppointmentReminderEventFailedParams{
		ID:        eventID,
		LastError: sql.NullString{String: errMsg, Valid: true},
	}); err != nil {
		logger.Warn("[reminder_job] failed to mark reminder event as failed",
			"event_id", eventID,
			"err", err)
	}
}
