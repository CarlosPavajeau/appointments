package state_machine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"wappiz/internal/services/slot_finder"
	"wappiz/pkg/date_formatter"
	"wappiz/pkg/date_parser"
	"wappiz/pkg/db"
	apperrors "wappiz/pkg/errors"
	"wappiz/pkg/logger"
	"wappiz/pkg/whatsapp"

	"github.com/google/uuid"
)

const (
	maxDateAttempts = 3
	sessionTTL      = 30 * time.Minute
	freePlanLimit   = 30
)

func (s *service) Process(ctx context.Context, msg IncomingMessage) error {
	logger.Info("[scheduling] processing message",
		"tenant_id", msg.TenantID,
		"from", msg.From,
		"body", msg.Body,
		"interactive_id", msg.InteractiveID)

	customer, err := db.Query.FindCustomerByPhoneNumber(ctx, s.db.Primary(), db.FindCustomerByPhoneNumberParams{
		TenantID:    msg.TenantID,
		PhoneNumber: msg.From,
	})
	if err != nil {
		logger.Info("[scheduling] customer not found, creating new one",
			"tenant_id", msg.TenantID,
			"phone_number", msg.From)

		if err := db.Query.InsertCustomer(ctx, s.db.Primary(), db.InsertCustomerParams{
			TenantID:    msg.TenantID,
			PhoneNumber: msg.From,
		}); err != nil {
			logger.Error("[scheduling] failed to create customer",
				"tenant_id", msg.TenantID,
				"phone_number", msg.From,
				"err", err)
			return err
		}

		customer, err = db.Query.FindCustomerByPhoneNumber(ctx, s.db.Primary(), db.FindCustomerByPhoneNumberParams{
			TenantID:    msg.TenantID,
			PhoneNumber: msg.From,
		})

		if err != nil {
			logger.Error("[scheduling] failed to retrieve newly created customer",
				"tenant_id", msg.TenantID,
				"phone_number", msg.From,
				"err", err)
			return err
		}
	}

	if customer.IsBlocked {
		logger.Info("[scheduling] customer is blocked, ignoring message",
			"tenant_id", msg.TenantID,
			"phone_number", msg.From)
		return nil
	}

	session, err := db.Query.FindCustomerActiveConversationSession(ctx, s.db.Primary(), db.FindCustomerActiveConversationSessionParams{
		TenantID:   msg.TenantID,
		CustomerID: customer.ID,
	})

	if err != nil { // TODO: Assume session don't exist, improve this check
		return s.handleEntry(ctx, msg, customer)
	}

	switch SessionStep(session.Step) {
	case StepSelectService:
		return s.handleSelectService(ctx, msg, session)

	case StepSelectResource:
		return s.handleSelectResource(ctx, msg, session)

	case StepSelectDate:
		return s.handleSelectDate(ctx, msg, session, customer)

	case StepSelectTime:
		return s.handleSelectTime(ctx, msg, session, customer)

	case StepAwaitingName:
		return s.handleAwaitingName(ctx, msg, session, customer)

	case StepConfirm:
		return s.handleConfirm(ctx, msg, session, customer)

	default:
		logger.Warn("[scheduling] unknown step "+session.Step+" resetting to entry",
			"session_id", session.ID)

		if err := db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID); err != nil {
			logger.Warn("[scheduling] failed to delete session with unknown step, resetting to entry",
				"session_id", session.ID,
				"err", err)

		}

		return s.handleEntry(ctx, msg, customer)
	}
}

func (s *service) handleEntry(ctx context.Context, msg IncomingMessage, customer db.FindCustomerByPhoneNumberRow) error {
	if msg.InteractiveID != nil {
		interactiveID := *msg.InteractiveID

		switch {
		case interactiveID == "action_schedule":
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

		case interactiveID == "action_my_appointments":
			return s.handleMyAppointments(ctx, msg, customer)

		case interactiveID == "action_cancel":
			return s.handleCancelFlow(ctx, msg, customer)

		case strings.HasPrefix(interactiveID, "cancel_"):
			return s.handleCancelConfirm(ctx, msg, customer)

		case strings.HasPrefix(interactiveID, "confirm_cancel_"):
			return s.handleCancelExecute(ctx, msg, customer)

		case interactiveID == "action_keep":
			return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, "👍 Perfecto, tu cita sigue agendada. ¿Hay algo más en lo que pueda ayudarte?")
		default:
			logger.Warn("[scheduling] unknown interactive ID on entry step, ignoring",
				"tenant_id", msg.TenantID,
				"interactive_id", interactiveID)
		}
	}

	tenant, err := db.Query.FindTenantByID(ctx, s.db.Primary(), msg.TenantID)
	if err != nil {
		logger.Error("[scheduling] failed to find tenant for entry step",
			"tenant_id", msg.TenantID,
			"err", err)
		return fmt.Errorf("find tenant: %w", err)
	}

	var tenantSettings db.TenantSettings
	if err := json.Unmarshal(tenant.Settings, &tenantSettings); err != nil {
		logger.Warn("[scheduling] failed to unmarshal tenant settings",
			"err", err)
		return err
	}

	var welcomeMsg string
	if len(tenantSettings.WelcomeMessage) > 0 {
		welcomeMsg = tenantSettings.WelcomeMessage
	} else {
		welcomeMsg = "¡Hola! Bienvenido a *" + tenant.Name + "*"
	}

	body := "👋 " + welcomeMsg + "\n\n¿Qué deseas hacer?"
	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_schedule", Title: "📅 Agendar cita"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_my_appointments", Title: "📋 Mis citas"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_cancel", Title: "❌ Cancelar cita"}},
	}

	return s.whatsapp.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

func (s *service) handleSelectService(ctx context.Context, msg IncomingMessage, session db.ConversationSession) error {
	svc, err := s.validateService(ctx, msg.TenantID, msg.InteractiveID)
	if err != nil {
		return s.sendServiceList(ctx, msg)
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		logger.Error("[scheduling] failed to unmarshal session data on select service step",
			"session_id", session.ID,
			"err", err)
		return s.sendServiceList(ctx, msg)
	}

	sessionData.ServiceID = &svc.ID
	session.Step = string(StepSelectResource)
	session.ExpiresAt = time.Now().Add(sessionTTL)

	session, err = s.updateSession(ctx, session, sessionData)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rsc, err := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
		TenantID:  session.TenantID,
		ServiceID: svc.ID,
	})

	if err != nil {
		return fmt.Errorf("find resources: %w", err)
	}

	if len(rsc) == 1 {
		sessionData.ResourceID = &rsc[0].ID
		session.Step = string(StepSelectDate)

		session, err = s.updateSession(ctx, session, sessionData)
		if err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		return s.sendDatePrompt(ctx, msg)
	}

	return s.sendResourceList(ctx, msg, rsc)
}

func (s *service) validateService(ctx context.Context, tenantID uuid.UUID, interactiveID *string) (*db.Service, error) {
	if interactiveID == nil {
		return nil, apperrors.ErrInvalidFormat
	}

	serviceID, err := uuid.Parse(*interactiveID)
	if err != nil {
		return nil, apperrors.ErrInvalidFormat
	}

	svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), serviceID)
	if err != nil {
		return nil, apperrors.ErrNotFound
	}

	if svc.TenantID != tenantID {
		return nil, apperrors.ErrNotFound
	}

	return &svc, nil
}

func (s *service) handleSelectResource(ctx context.Context, msg IncomingMessage, session db.ConversationSession) error {
	interactiveID := msg.InteractiveID

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		logger.Error("[scheduling] failed to marshal session data on select resource step",
			"session_id", session.ID,
			"err", err)
		return s.sendServiceList(ctx, msg)
	}

	if interactiveID == nil {
		rsc, _ := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
			TenantID:  session.TenantID,
			ServiceID: *sessionData.ServiceID,
		})

		return s.sendResourceList(ctx, msg, rsc)
	}

	var resourceID *uuid.UUID
	if *interactiveID == "resource_any" {
		resourceID = nil
	} else {
		id, err := uuid.Parse(*interactiveID)
		if err != nil {
			rsc, _ := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
				TenantID:  session.TenantID,
				ServiceID: *sessionData.ServiceID,
			})

			return s.sendResourceList(ctx, msg, rsc)
		}

		resourceID = &id
	}

	sessionData.ResourceID = resourceID
	session.Step = string(StepSelectDate)

	if _, err := s.updateSession(ctx, session, sessionData); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.sendDatePrompt(ctx, msg)
}

func (s *service) updateSession(ctx context.Context, session db.ConversationSession, sessionData SessionData) (db.ConversationSession, error) {
	updatedData, err := json.Marshal(sessionData)
	if err != nil {
		return session, fmt.Errorf("marshal session data: %w", err)
	}

	session.Data = updatedData

	if err := db.Query.UpdateConversationSession(ctx, s.db.Primary(), db.UpdateConversationSessionParams{
		Step:      session.Step,
		Data:      session.Data,
		ExpiresAt: session.ExpiresAt,
		ID:        session.ID,
	}); err != nil {
		return session, fmt.Errorf("update session: %w", err)
	}

	return session, nil
}

func (s *service) handleSelectDate(ctx context.Context, msg IncomingMessage, session db.ConversationSession, customer db.FindCustomerByPhoneNumberRow) error {
	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		logger.Error("[scheduling] failed to marshal session data on select resource step",
			"session_id", session.ID,
			"err", err)
		return fmt.Errorf("unmarshal session data: %w", err)
	}

	if sessionData.DateAttempts >= maxDateAttempts {
		logger.Warn("[scheduling] max date attempts reached, resetting session",
			"session_id", session.ID)

		_ = db.Query.DeleteConversationSession(ctx, s.db.Primary(), session.ID)

		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Parece que estás teniendo problemas para agendar 😅\n"+
				"Escríbenos cuando quieras e intentamos de nuevo.\n\n"+
				"Escribe *hola* para comenzar.")
	}

	tenant, err := db.Query.FindTenantByID(ctx, s.db.Primary(), session.TenantID)
	if err != nil {
		return fmt.Errorf("find tenant by id: %w", err)
	}

	result, err := s.validateAndFindSlots(ctx, msg.Body, tenant.Timezone, session)

	if err != nil {
		sessionData.DateAttempts++
		if _, err := s.updateSession(ctx, session, sessionData); err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		errMsg := buildErrorMessage(err, msg.Body, nil)

		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg)
	}

	if !result.SlotTaken {
		sessionData.StartsAt = &result.StartsAt
		if result.ResourceID != nil {
			sessionData.ResourceID = result.ResourceID
		}
		sessionData.DateAttempts = 0

		return s.advanceToConfirmOrName(ctx, msg, session, sessionData, customer)
	}

	if len(result.Slots) == 0 {
		sessionData.DateAttempts++

		if _, err = s.updateSession(ctx, session, sessionData); err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos disponibilidad cerca a esa fecha 😔\nPor favor intenta con otra fecha.")
	}

	session.Step = string(StepSelectTime)
	sessionData.DateAttempts = 0

	if _, err = s.updateSession(ctx, session, sessionData); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.sendSlotList(ctx, msg, result.Slots)
}

func (s *service) validateAndFindSlots(ctx context.Context, input, timezone string, session db.ConversationSession) (*DateValidationResult, error) {
	loc, _ := time.LoadLocation(timezone)
	t, err := date_parser.ParseDateTime(input, loc)
	if err != nil {
		logger.Warn("[scheduling] failed to parse date input",
			"input", input,
			"err", err)
		return nil, err
	}

	if t.Before(time.Now()) {
		return nil, apperrors.ErrDateInPast
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		return nil, err
	}

	svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), *sessionData.ServiceID)
	if err != nil {
		return nil, err
	}

	if sessionData.ResourceID != nil {
		slots, err := s.slotFinder.FindAvailableSlots(ctx, slot_finder.FindAvailableSlotsParams{
			ResourceID: *sessionData.ResourceID,
			Date:       t,
			Service: slot_finder.ServiceParam{
				DurationMinutes: svc.DurationMinutes,
				BufferMinutes:   svc.BufferMinutes,
			},
		})

		if err != nil {
			return nil, err
		}

		if len(slots) == 0 {
			return nil, apperrors.ErrDayOff
		}

		for _, slot := range slots {
			if slot.StartsAt.Equal(t) {
				resourceID := *sessionData.ResourceID
				return &DateValidationResult{
					StartsAt:   t,
					ResourceID: &resourceID,
				}, nil
			}
		}

		suggestions, err := s.slotFinder.GetSuggestedSlots(ctx, slot_finder.GetSuggestedSlotsParams{
			ResourceID: *sessionData.ResourceID,
			From:       t,
			Service: slot_finder.ServiceParam{
				DurationMinutes: svc.DurationMinutes,
				BufferMinutes:   svc.BufferMinutes,
			},
		})

		if err != nil {
			return nil, err
		}

		return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: suggestions}, nil
	}

	rsc, err := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
		TenantID:  session.TenantID,
		ServiceID: *sessionData.ServiceID,
	})

	if err != nil {
		return nil, err
	}

	for _, res := range rsc {
		slots, err := s.slotFinder.FindAvailableSlots(ctx, slot_finder.FindAvailableSlotsParams{
			ResourceID: res.ID,
			Date:       t,
			Service: slot_finder.ServiceParam{
				DurationMinutes: svc.DurationMinutes,
				BufferMinutes:   svc.BufferMinutes,
			},
		})

		if err != nil {
			continue
		}

		for _, slot := range slots {
			if slot.StartsAt.Equal(t) {
				resourceID := res.ID
				return &DateValidationResult{
					StartsAt:   t,
					ResourceID: &resourceID,
				}, nil
			}
		}
	}

	// Find suggestions across all resources
	var allSuggestions []slot_finder.TimeSlot
	for _, res := range rsc {
		suggestions, _ := s.slotFinder.GetSuggestedSlots(ctx, slot_finder.GetSuggestedSlotsParams{
			ResourceID: res.ID,
			From:       t,
			Service: slot_finder.ServiceParam{
				DurationMinutes: svc.DurationMinutes,
				BufferMinutes:   svc.BufferMinutes,
			},
		})

		allSuggestions = append(allSuggestions, suggestions...)

		if len(allSuggestions) >= 3 {
			break
		}
	}

	return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: allSuggestions}, nil
}

func (s *service) handleSelectTime(ctx context.Context, msg IncomingMessage, session db.ConversationSession, customer db.FindCustomerByPhoneNumberRow) error {
	interactiveID := msg.InteractiveID
	if interactiveID == nil {
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor selecciona una de las opciones de la lista 👆")
	}

	startsAt, resourceID, err := parseSlotID(*interactiveID)
	if err != nil {
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor selecciona una de las opciones de la lista 👆")
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		return err
	}

	sessionData.StartsAt = &startsAt
	sessionData.ResourceID = &resourceID

	return s.advanceToConfirmOrName(ctx, msg, session, sessionData, customer)
}

func (s *service) handleAwaitingName(ctx context.Context, msg IncomingMessage, session db.ConversationSession, customer db.FindCustomerByPhoneNumberRow) error {
	name := strings.TrimSpace(msg.Body)
	if len(name) < 2 {
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor dinos tu nombre para continuar 😊")
	}

	customer.Name = sql.NullString{String: name, Valid: true}

	if err := db.Query.UpdateCustomer(ctx, s.db.Primary(), db.UpdateCustomerParams{
		Name: customer.Name,
		ID:   customer.ID,
	}); err != nil {
		return err
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		return err
	}

	sessionData.ConfirmedName = &name
	session.Step = string(StepConfirm)

	var err error
	session, err = s.updateSession(ctx, session, sessionData)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.sendConfirmation(ctx, msg, session)
}

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
			if strings.Contains(err.Error(), "no_overlap") || strings.Contains(err.Error(), "exclusion constraint") {
				logger.Warn("[scheduling] appointment overlap detected on confirm, informing customer",
					"session_id", session.ID)

				suggestions, err := s.slotFinder.GetSuggestedSlots(ctx, slot_finder.GetSuggestedSlotsParams{
					ResourceID: *sessionData.ResourceID,
					From:       *sessionData.StartsAt,
					Service: slot_finder.ServiceParam{
						DurationMinutes: svc.DurationMinutes,
						BufferMinutes:   svc.BufferMinutes,
					},
				})

				if err != nil {
					return err
				}

				session.Step = string(StepSelectTime)

				if _, err = s.updateSession(ctx, session, sessionData); err != nil {
					return fmt.Errorf("update session: %w", err)
				}

				errMsg := buildErrorMessage(apperrors.ErrOverlap, "", suggestions)
				if err := s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg); err != nil {
					return err
				}

				return s.sendSlotList(ctx, msg, suggestions)
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

		return s.sendAppointmentConfirmed(ctx, msg, appointmentID, session, customer)

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

func (s *service) handleMyAppointments(ctx context.Context, msg IncomingMessage, customer db.FindCustomerByPhoneNumberRow) error {
	appt, err := db.Query.FindAppointmentsByCustomerID(ctx, s.db.Primary(), db.FindAppointmentsByCustomerIDParams{
		TenantID:   msg.TenantID,
		CustomerID: customer.ID,
	})

	if err != nil {
		return fmt.Errorf("find appointments: %w", err)
	}

	if len(appt) == 0 {
		buttons := []whatsapp.Button{
			{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_schedule", Title: "📅 Agendar cita"}},
		}

		return s.whatsapp.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No tienes citas próximas agendadas 📭\n¿Deseas agendar una?", buttons)
	}

	var customerName string
	if customer.Name.Valid {
		customerName = customer.Name.String
	} else {
		customerName = customer.PhoneNumber
	}

	text := fmt.Sprintf("¡Hola, %s! 👋 Aquí están tus próximas citas:\n", customerName)
	for i, a := range appt {
		date := date_formatter.FormatTime(a.StartsAt, "Monday 02 Jan")
		timeStr := date_formatter.FormatTime(a.StartsAt, "03:04 PM")

		text += fmt.Sprintf("\n*%d.* 📌 *%s*\n", i+1, a.ServiceName)
		text += fmt.Sprintf("   🗓️ %s a las %s\n", date, timeStr)
		text += fmt.Sprintf("   👤 Con: %s\n", a.ResourceName)
		text += fmt.Sprintf("   📊 Estado: %s\n", appointmentStatusLabel(string(a.Status)))
	}

	text += "\n💬 Para cancelar una cita, toca el botón *Cancelar cita* en el menú principal."

	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, text)
}

func appointmentStatusLabel(status string) string {
	switch status {
	case "scheduled":
		return "Agendada ✅"
	case "confirmed":
		return "Confirmada ✅"
	case "checked_in":
		return "En proceso 🔄"
	case "completed":
		return "Completada 🎉"
	case "cancelled":
		return "Cancelada ❌"
	case "no_show":
		return "No asistió ⚠️"
	default:
		return status
	}
}

func (s *service) handleCancelFlow(ctx context.Context, msg IncomingMessage, customer db.FindCustomerByPhoneNumberRow) error {
	appt, err := db.Query.FindAppointmentsByCustomerID(ctx, s.db.Primary(), db.FindAppointmentsByCustomerIDParams{
		TenantID:   msg.TenantID,
		CustomerID: customer.ID,
	})

	if err != nil {
		return fmt.Errorf("find appointments: %w", err)
	}

	if len(appt) == 0 {
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No tienes citas activas para cancelar 📭")
	}

	var rows []whatsapp.ListRow
	for _, a := range appt {
		rows = append(rows, whatsapp.ListRow{
			ID:          "cancel_" + a.ID.String(),
			Title:       date_formatter.FormatTime(a.StartsAt, "02/01 03:04 PM"),
			Description: fmt.Sprintf("%s · %s", a.ResourceName, a.ServiceName),
		})
	}

	sections := []whatsapp.Section{
		{
			Title: "Elige una cita",
			Rows:  rows,
		},
	}

	return s.whatsapp.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Cuál cita deseas cancelar? 🗓️", sections)
}

func (s *service) handleCancelConfirm(ctx context.Context, msg IncomingMessage, customer db.FindCustomerByPhoneNumberRow) error {
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

	svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), appointment.ServiceID)
	if err != nil {
		logger.Warn("[scheduling] failed to find service for cancel confirmation",
			"appointment_id", appointmentID,
			"service_id", appointment.ServiceID,
			"err", err)
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	rsc, err := db.Query.FindResourceById(ctx, s.db.Primary(), appointment.ResourceID)
	if err != nil {
		logger.Warn("[scheduling] failed to find resource for cancel confirmation",
			"appointment_id", appointmentID,
			"resource_id", appointment.ResourceID,
			"err", err)
		return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	body := fmt.Sprintf(
		"¿Confirmas la cancelación de esta cita? 🗓️\n\n"+
			"%s con %s\n"+
			"📅 %s\n\n"+
			"Esta acción no se puede deshacer.",
		svc.Name, rsc.Name,
		date_formatter.FormatTime(appointment.StartsAt, "02/01/2006 03:04 PM"),
	)
	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_cancel_" + appointmentID.String(), Title: "✅ Sí, cancelar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_keep", Title: "🔙 No, mantener"}},
	}

	return s.whatsapp.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

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

func (s *service) sendServiceList(ctx context.Context, msg IncomingMessage) error {
	svcs, err := db.Query.FindServicesWithAssignedResourceByTenantID(ctx, s.db.Primary(), msg.TenantID)
	if err != nil {
		return err
	}

	var rows []whatsapp.ListRow
	for _, svc := range svcs {
		price, _ := strconv.ParseFloat(svc.Price, 64)
		rows = append(rows, whatsapp.ListRow{
			ID:          svc.ID.String(),
			Title:       svc.Name,
			Description: fmt.Sprintf("%d min · $%.0f", svc.DurationMinutes, price),
		})
	}

	sections := []whatsapp.Section{
		{
			Title: "Servicios disponibles",
			Rows:  rows,
		},
	}

	return s.whatsapp.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Qué servicio deseas?", sections)
}

func (s *service) sendResourceList(ctx context.Context, msg IncomingMessage, resources []db.FindResourcesByServiceIDRow) error {
	var rows []whatsapp.ListRow
	for _, resource := range resources {
		rows = append(rows, whatsapp.ListRow{
			ID:    resource.ID.String(),
			Title: resource.Name,
		})
	}

	rows = append(rows, whatsapp.ListRow{
		ID:          "resource_any",
		Title:       "Sin preferencia",
		Description: "Te asignamos el primero disponible",
	})

	sections := []whatsapp.Section{
		{
			Title: "Recursos disponibles",
			Rows:  rows,
		},
	}

	return s.whatsapp.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Con quién deseas tu cita?", sections)
}

func (s *service) sendDatePrompt(ctx context.Context, msg IncomingMessage) error {
	body := "¿Para qué fecha y hora deseas tu cita? 📅\n\n" +
		"Escribe en este formato:\n*DD/MM HH:mm AM/PM*\n\n" +
		"Ejemplo: *15/03 09:00 AM*\n\n" +
		"Atendemos de lunes a sábado, 9:00 AM – 7:00 PM."
	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body)
}

func (s *service) sendSlotList(ctx context.Context, msg IncomingMessage, slots []slot_finder.TimeSlot) error {
	var rows []whatsapp.ListRow
	for _, slot := range slots {
		rows = append(rows, whatsapp.ListRow{
			ID:          buildSlotID(slot),
			Title:       slot.StartsAt.Format("02/01 03:04 PM"),
			Description: slot.ResourceName,
		})
	}

	sections := []whatsapp.Section{
		{
			Title: "Horarios disponibles",
			Rows:  rows,
		},
	}

	return s.whatsapp.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"Elige un horario disponible 🕐", sections)
}

func (s *service) sendConfirmation(ctx context.Context, msg IncomingMessage, session db.ConversationSession) error {
	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		return err
	}

	if sessionData.ServiceID == nil || sessionData.ResourceID == nil || sessionData.StartsAt == nil {
		return fmt.Errorf("incomplete session data: missing service, resource, or start time")
	}

	svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), *sessionData.ServiceID)
	if err != nil {
		return err
	}

	rsc, err := db.Query.FindResourceById(ctx, s.db.Primary(), *sessionData.ResourceID)
	if err != nil {
		return err
	}

	customerName := "Cliente"
	if sessionData.ConfirmedName != nil {
		customerName = *sessionData.ConfirmedName
	}

	body := fmt.Sprintf(
		"Resumen de tu cita 📋\n\n"+
			"👤 Cliente:  %s\n"+
			"📌 Servicio: %s (%d min)\n"+
			"💈 Barbero:  %s\n"+
			"📅 Fecha:    %s\n"+
			"💰 Precio:   $%s\n\n"+
			"¿Confirmamos?",
		customerName,
		svc.Name,
		svc.DurationMinutes,
		rsc.Name,
		date_formatter.FormatTime(*sessionData.StartsAt, "02/01/2006 03:04 PM"),
		svc.Price,
	)

	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_yes", Title: "✅ Confirmar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_modify", Title: "✏️ Modificar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_cancel", Title: "❌ Cancelar"}},
	}

	return s.whatsapp.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

func (s *service) sendAppointmentConfirmed(ctx context.Context, msg IncomingMessage, appointmentID uuid.UUID, session db.ConversationSession, customer db.FindCustomerByPhoneNumberRow) error {
	appt, err := db.Query.FindAppointmentByID(ctx, s.db.Primary(), db.FindAppointmentByIDParams{
		ID:       appointmentID,
		TenantID: msg.TenantID,
	})

	if err != nil {
		return err
	}

	svc, err := db.Query.FindServiceByID(ctx, s.db.Primary(), appt.ServiceID)
	if err != nil {
		return err
	}

	rsc, err := db.Query.FindResourceById(ctx, s.db.Primary(), appt.ResourceID)
	if err != nil {
		return err
	}

	customerName := "Cliente"
	if customer.Name.Valid {
		customerName = customer.Name.String
	}

	body := fmt.Sprintf(
		"¡Listo, %s! 🎉 Tu cita está confirmada.\n\n"+
			"📌 %s con %s\n"+
			"📅 %s\n"+
			"Te enviaremos un recordatorio 24 horas antes.\n"+
			"Si necesitas cancelar escríbenos aquí. ¡Hasta pronto! 👋",
		customerName,
		svc.Name,
		rsc.Name,
		date_formatter.FormatTime(appt.StartsAt, "02/01/2006 03:04 PM"),
	)

	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body)
}

func (s *service) advanceToConfirmOrName(ctx context.Context, msg IncomingMessage, session db.ConversationSession, sessionData SessionData, customer db.FindCustomerByPhoneNumberRow) error {
	var err error
	if customer.Name.Valid {
		sessionData.ConfirmedName = new(customer.Name.String)
		session.Step = string(StepConfirm)

		session, err = s.updateSession(ctx, session, sessionData)
		if err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		return s.sendConfirmation(ctx, msg, session)
	}

	session.Step = string(StepAwaitingName)
	if _, err = s.updateSession(ctx, session, sessionData); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"Antes de confirmar, ¿cuál es tu nombre? 😊")
}

func buildErrorMessage(err error, input string, suggestions []slot_finder.TimeSlot) string {
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
