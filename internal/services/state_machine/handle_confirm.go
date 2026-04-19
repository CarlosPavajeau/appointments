package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wappiz/pkg/db"
	apperrors "wappiz/pkg/errors"
	"wappiz/pkg/logger"

	"github.com/google/uuid"
)

func (s *service) handleConfirm(ctx context.Context, msg IncomingMessage, session db.ConversationSession, customer db.FindCustomerByPhoneNumberRow) error {
	interactiveID := msg.InteractiveID
	if interactiveID == nil {
		return s.sendConfirmation(ctx, msg, session)
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		return err
	}

	switch *interactiveID {
	case "confirm_yes":
		tenant, err := db.Query.FindTenantByID(ctx, s.db.Primary(), session.TenantID)
		if err != nil {
			return fmt.Errorf("find tenant by id: %w", err)
		}

		if tenant.Plan == "free" && tenant.AppointmentsThisMonth >= freePlanLimit {
			return apperrors.ErrPlanLimitReached
		}

		svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), *sessionData.ServiceID)
		if err != nil {
			return fmt.Errorf("find service by id: %w", err)
		}

		startsAt := *sessionData.StartsAt
		endsAt := startsAt.Add(time.Duration(svc.DurationMinutes) * time.Minute)
		appointmentID := uuid.New()

		hasCustomerOverlap, err := s.hasCustomerOverlap(ctx, tenant.ID, session.CustomerID, startsAt, endsAt)
		if err != nil {
			return fmt.Errorf("check customer overlap: %w", err)
		}
		if hasCustomerOverlap {
			logger.Warn("[scheduling] customer overlap detected on confirm, informing customer",
				"session_id", session.ID,
				"customer_id", session.CustomerID)
			return s.handleOverlapOnConfirm(ctx, msg, session, sessionData, svc)
		}

		if err := db.Query.InsertAppointment(ctx, s.db.Primary(), db.InsertAppointmentParams{
			ID:             appointmentID,
			TenantID:       tenant.ID,
			ResourceID:     *sessionData.ResourceID,
			ServiceID:      *sessionData.ServiceID,
			CustomerID:     session.CustomerID,
			StartsAt:       startsAt,
			EndsAt:         endsAt,
			PriceAtBooking: svc.Price,
		}); err != nil {
			// The DB exclusion constraints are the authoritative source for overlap checks.
			if isAppointmentOverlapConstraintError(err) {
				logger.Warn("[scheduling] appointment overlap detected on confirm, informing customer",
					"session_id", session.ID,
					"err", err)
				return s.handleOverlapOnConfirm(ctx, msg, session, sessionData, svc)
			}
			return fmt.Errorf("insert appointment: %w", err)
		}

		if err := db.Query.UpdateTenantAppointmentCount(ctx, s.db.Primary(), db.UpdateTenantAppointmentCountParams{
			ID:                    tenant.ID,
			AppointmentsThisMonth: tenant.AppointmentsThisMonth + 1,
		}); err != nil {
			return fmt.Errorf("update tenant appointment count: %w", err)
		}

		if err := db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID); err != nil {
			logger.Warn("[scheduling] failed to delete session after confirming appointment",
				"session_id", session.ID,
				"err", err)
		}

		return s.sendAppointmentConfirmed(ctx, msg, appointmentID, customer)

	case "confirm_modify":
		if err := db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID); err != nil {
			logger.Warn("[scheduling] failed to delete session after confirming modify",
				"session_id", session.ID,
				"err", err)
			return fmt.Errorf("delete session after confirm_modify: %w", err)
		}

		sessionID := uuid.New()
		if err := db.Query.InsertConversationSession(ctx, s.db.Primary(), db.InsertConversationSessionParams{
			ID:               sessionID,
			TenantID:         msg.TenantID,
			WhatsappConfigID: msg.WhatsappConfigID,
			CustomerID:       customer.ID,
			Step:             string(StepSelectService),
			Data:             json.RawMessage("{}"),
			ExpiresAt:        time.Now().Add(sessionTTL),
		}); err != nil {
			return fmt.Errorf("create session: %w", err)
		}

		return s.sendServiceList(ctx, msg)

	case "confirm_cancel":
		if err := db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID); err != nil {
			logger.Warn("[scheduling] failed to delete session after confirming cancel",
				"session_id", session.ID,
				"err", err)
			return fmt.Errorf("delete session after confirming cancel: %w", err)
		}

		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Entendido, hemos cancelado el proceso 👋\nEscríbenos cuando quieras agendar.")
	}

	return s.sendConfirmation(ctx, msg, session)
}
