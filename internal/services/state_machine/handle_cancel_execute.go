package state_machine

import (
	"context"
	"database/sql"
	"strings"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"

	"github.com/google/uuid"
)

func (s *service) handleCancelExecute(ctx context.Context, msg IncomingMessage, customer db.FindCustomerByPhoneNumberRow) error {
	appointmentID, err := uuid.Parse(strings.TrimPrefix(*msg.InteractiveID, "cancel_"))
	if err != nil {
		logger.Warn("[scheduling] failed to parse interactive id from cancel confirmation",
			"interactive_id", *msg.InteractiveID,
			"err", err)

		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	appointment, err := db.Query.FindAppointmentByID(ctx, s.db.Primary(), db.FindAppointmentByIDParams{
		ID:       appointmentID,
		TenantID: msg.TenantID,
	})

	if err != nil {
		logger.Warn("[scheduling] failed to find appointment for cancel confirmation",
			"appointment_id", appointmentID,
			"err", err)
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	if appointment.CustomerID != customer.ID {
		logger.Warn("[scheduling] appointment does not belong to customer for cancel confirmation",
			"appointment_id", appointmentID,
			"customer_id", customer.ID)
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	if err := db.Query.UpdateAppointment(ctx, s.db.Primary(), db.UpdateAppointmentParams{
		Status:       db.AppointmentStatusCancelled,
		CancelledBy:  sql.NullString{},
		CancelReason: sql.NullString{String: "Cancelado por el cliente", Valid: true},
		CompletedAt:  sql.NullTime{},
		ID:           appointmentID,
	}); err != nil {
		logger.Warn("[scheduling] failed to update appointment status to cancelled",
			"appointment_id", appointmentID,
			"err", err)
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error al cancelar. Por favor intenta de nuevo.")
	}

	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"✅ Tu cita ha sido cancelada. Si deseas agendar una nueva cita, no dudes en escribirnos.")
}
