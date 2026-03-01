package scheduling

import (
	"appointments/internal/features/customers"
	"appointments/internal/features/resources"
	"appointments/internal/features/tenants"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"appointments/internal/platform/whatsapp"
	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
)

type StateMachine struct {
	sessions   SessionRepository
	useCases   *UseCases
	wa         whatsapp.Client
	tenantRepo tenants.Repository
}

func NewStateMachine(sessions SessionRepository, uc *UseCases, wa whatsapp.Client, tr tenants.Repository) *StateMachine {
	return &StateMachine{sessions: sessions, useCases: uc, wa: wa, tenantRepo: tr}
}

func (sm *StateMachine) Process(ctx context.Context, msg IncomingMessage) error {
	log.Printf("[scheduling] processing message | tenant=%s from=%s body=%q interactiveID=%v",
		msg.TenantID, msg.From, msg.Body, msg.InteractiveID)

	log.Printf("[scheduling] resolving customer | tenant=%s phone=%s", msg.TenantID, msg.From)
	customer, err := sm.useCases.ResolveCustomer(ctx, msg.TenantID, msg.From)
	if err != nil {
		log.Printf("[scheduling] ERROR resolving customer | tenant=%s phone=%s err=%v",
			msg.TenantID, msg.From, err)
		return fmt.Errorf("resolve customer: %w", err)
	}
	log.Printf("[scheduling] customer resolved | id=%s name=%v blocked=%v",
		customer.ID, customer.Name, customer.IsBlocked)

	if customer.IsBlocked {
		log.Printf("[scheduling] customer is blocked, ignoring | id=%s", customer.ID)
		return nil
	}

	log.Printf("[scheduling] looking for active session | tenant=%s customerID=%s",
		msg.TenantID, customer.ID)
	session, err := sm.sessions.FindActive(ctx, msg.TenantID, customer.ID)
	if err != nil && !errors.Is(err, apperrors.ErrSessionNotFound) {
		log.Printf("[scheduling] ERROR finding session | tenant=%s customerID=%s err=%v",
			msg.TenantID, customer.ID, err)
		return fmt.Errorf("find session: %w", err)
	}

	if session == nil {
		log.Printf("[scheduling] no active session found, starting entry flow | customerID=%s", customer.ID)
		return sm.handleEntry(ctx, msg, customer)
	}

	log.Printf("[scheduling] active session found | sessionID=%s step=%s data=%+v expiresAt=%s",
		session.ID, session.Step, session.Data, session.ExpiresAt.Format(time.RFC3339))

	switch session.Step {
	case StepSelectService:
		log.Printf("[scheduling] dispatching to handleSelectService | sessionID=%s", session.ID)
		return sm.handleSelectService(ctx, msg, session)

	case StepSelectResource:
		log.Printf("[scheduling] dispatching to handleSelectResource | sessionID=%s", session.ID)
		return sm.handleSelectResource(ctx, msg, session)

	case StepSelectDate:
		log.Printf("[scheduling] dispatching to handleSelectDate | sessionID=%s", session.ID)
		return sm.handleSelectDate(ctx, msg, session, customer)

	case StepSelectTime:
		log.Printf("[scheduling] dispatching to handleSelectTime | sessionID=%s", session.ID)
		return sm.handleSelectTime(ctx, msg, session, customer)

	case StepAwaitingName:
		log.Printf("[scheduling] dispatching to handleAwaitingName | sessionID=%s", session.ID)
		return sm.handleAwaitingName(ctx, msg, session, customer)

	case StepConfirm:
		log.Printf("[scheduling] dispatching to handleConfirm | sessionID=%s", session.ID)
		return sm.handleConfirm(ctx, msg, session, customer)

	default:
		log.Printf("[scheduling] unknown step %q, resetting to entry | sessionID=%s", session.Step, session.ID)
		sm.sessions.Delete(ctx, session.ID)
		return sm.handleEntry(ctx, msg, customer)
	}
}

func (sm *StateMachine) handleEntry(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	log.Printf("[scheduling] handleEntry | customerID=%s interactiveID=%v", customer.ID, msg.InteractiveID)

	if msg.InteractiveID != nil {
		log.Printf("[scheduling] entry action received | interactiveID=%s", *msg.InteractiveID)

		switch {
		case *msg.InteractiveID == "action_schedule":
			log.Printf("[scheduling] creating new session | customerID=%s", customer.ID)
			session, err := sm.useCases.CreateSession(ctx, msg.TenantID, msg.WhatsappConfigID, customer.ID)
			if err != nil {
				log.Printf("[scheduling] ERROR creating session | customerID=%s err=%v", customer.ID, err)
				return fmt.Errorf("create session: %w", err)
			}
			log.Printf("[scheduling] session created | sessionID=%s step=%s", session.ID, session.Step)
			return sm.sendServiceList(ctx, msg, session)

		case *msg.InteractiveID == "action_my_appointments":
			return sm.handleMyAppointments(ctx, msg, customer)

		case *msg.InteractiveID == "action_cancel":
			return sm.handleCancelFlow(ctx, msg, customer)

		case strings.HasPrefix(*msg.InteractiveID, "cancel_"):
			return sm.handleCancelConfirm(ctx, msg, customer)

		case strings.HasPrefix(*msg.InteractiveID, "confirm_cancel_"):
			return sm.handleCancelExecute(ctx, msg, customer)

		case *msg.InteractiveID == "action_keep":
			log.Printf("[scheduling] customer chose to keep appointment | customerID=%s", customer.ID)
			return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
				"Tu cita sigue activa 👍\nEscríbenos si necesitas algo más.")
		default:
			log.Printf("[scheduling] unknown entry action %q, sending welcome | customerID=%s",
				*msg.InteractiveID, customer.ID)
		}
	}

	log.Printf("[scheduling] sending welcome menu | customerID=%s", customer.ID)

	tenant, err := sm.tenantRepo.FindByID(ctx, msg.TenantID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding tenant | tenantID=%s err=%v", msg.TenantID, err)
		return fmt.Errorf("find tenant: %w", err)
	}

	body := fmt.Sprintf("👋 ¡Hola! Bienvenido a *%s*\n\n¿Qué deseas hacer?", tenant.Name)
	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_schedule", Title: "📅 Agendar cita"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_my_appointments", Title: "📋 Mis citas"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_cancel", Title: "❌ Cancelar cita"}},
	}

	return sm.wa.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

func (sm *StateMachine) handleSelectService(ctx context.Context, msg IncomingMessage, session *Session) error {
	log.Printf("[scheduling] handleSelectService | sessionID=%s interactiveID=%v",
		session.ID, msg.InteractiveID)

	svc, err := sm.useCases.ValidateService(ctx, session.TenantID, msg.InteractiveID)
	if err != nil {
		log.Printf("[scheduling] invalid service selection | sessionID=%s interactiveID=%v err=%v",
			session.ID, msg.InteractiveID, err)
		return sm.sendServiceList(ctx, msg, session)
	}
	log.Printf("[scheduling] service selected | serviceID=%s name=%s", svc.ID, svc.Name)

	session.Data.ServiceID = &svc.ID
	session.Step = StepSelectResource
	if err := sm.useCases.AdvanceSession(ctx, session); err != nil {
		log.Printf("[scheduling] ERROR advancing session to SELECT_RESOURCE | sessionID=%s err=%v",
			session.ID, err)
		return fmt.Errorf("advance session: %w", err)
	}

	resources, err := sm.useCases.GetResourcesForService(ctx, session.TenantID, svc.ID)
	if err != nil {
		log.Printf("[scheduling] ERROR fetching resources | tenantID=%s serviceID=%s err=%v",
			session.TenantID, svc.ID, err)
		return fmt.Errorf("get resources: %w", err)
	}
	log.Printf("[scheduling] resources found | count=%d", len(resources))

	if len(resources) == 1 {
		log.Printf("[scheduling] single resource, skipping selection | resourceID=%s", resources[0].ID)
		session.Data.ResourceID = &resources[0].ID
		session.Step = StepSelectDate
		sm.useCases.AdvanceSession(ctx, session)
		return sm.sendDatePrompt(ctx, msg)
	}

	return sm.sendResourceList(ctx, msg, resources)
}

func (sm *StateMachine) handleSelectResource(ctx context.Context, msg IncomingMessage, session *Session) error {
	log.Printf("[scheduling] handleSelectResource | sessionID=%s interactiveID=%v",
		session.ID, msg.InteractiveID)

	if msg.InteractiveID == nil {
		log.Printf("[scheduling] no interactiveID, resending resource list | sessionID=%s", session.ID)
		resources, _ := sm.useCases.GetResourcesForService(ctx, session.TenantID, *session.Data.ServiceID)
		return sm.sendResourceList(ctx, msg, resources)
	}

	var resourceID *uuid.UUID
	if *msg.InteractiveID == "resource_any" {
		log.Printf("[scheduling] customer chose any resource | sessionID=%s", session.ID)
		resourceID = nil
	} else {
		id, err := uuid.Parse(*msg.InteractiveID)
		if err != nil {
			log.Printf("[scheduling] invalid resource uuid %q | sessionID=%s err=%v",
				*msg.InteractiveID, session.ID, err)
			resources, _ := sm.useCases.GetResourcesForService(ctx, session.TenantID, *session.Data.ServiceID)
			return sm.sendResourceList(ctx, msg, resources)
		}
		resourceID = &id
		log.Printf("[scheduling] resource selected | resourceID=%s", id)
	}

	session.Data.ResourceID = resourceID
	session.Step = StepSelectDate
	if err := sm.useCases.AdvanceSession(ctx, session); err != nil {
		log.Printf("[scheduling] ERROR advancing session to SELECT_DATE | sessionID=%s err=%v",
			session.ID, err)
		return fmt.Errorf("advance session: %w", err)
	}

	return sm.sendDatePrompt(ctx, msg)
}

func (sm *StateMachine) handleSelectDate(
	ctx context.Context,
	msg IncomingMessage,
	session *Session,
	customer *customers.Customer,
) error {
	log.Printf("[scheduling] handleSelectDate | sessionID=%s body=%q attempts=%d",
		session.ID, msg.Body, session.Data.DateAttempts)

	if session.Data.DateAttempts >= maxDateAttempts {
		log.Printf("[scheduling] max date attempts reached, resetting session | sessionID=%s", session.ID)
		sm.sessions.Delete(ctx, session.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Parece que estás teniendo problemas para agendar 😅\n"+
				"Escríbenos cuando quieras e intentamos de nuevo.\n\n"+
				"Escribe *hola* para comenzar.")
	}

	tenant, err := sm.tenantRepo.FindByID(ctx, session.TenantID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding tenant in handleSelectDate | tenantID=%s err=%v",
			session.TenantID, err)
		return fmt.Errorf("find tenant: %w", err)
	}

	log.Printf("[scheduling] validating date input | input=%q timezone=%s", msg.Body, tenant.Timezone)
	result, err := sm.useCases.ValidateAndFindSlots(ctx, msg.Body, tenant.Timezone, session)
	if err != nil {
		session.Data.DateAttempts++
		sm.useCases.AdvanceSession(ctx, session)
		errMsg := BuildErrorMessage(err, msg.Body, nil)
		log.Printf("[scheduling] date validation failed | sessionID=%s attempt=%d err=%v",
			session.ID, session.Data.DateAttempts, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg)
	}

	if !result.SlotTaken {
		log.Printf("[scheduling] exact slot available | sessionID=%s startsAt=%s",
			session.ID, result.StartsAt.Format(time.RFC3339))
		session.Data.StartsAt = &result.StartsAt
		session.Data.DateAttempts = 0
		return sm.advanceToConfirmOrName(ctx, msg, session, customer)
	}

	log.Printf("[scheduling] slot taken | sessionID=%s suggestions=%d",
		session.ID, len(result.Slots))

	if len(result.Slots) == 0 {
		session.Data.DateAttempts++
		sm.useCases.AdvanceSession(ctx, session)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos disponibilidad cerca a esa fecha 😔\nPor favor intenta con otra fecha.")
	}

	session.Step = StepSelectTime
	session.Data.DateAttempts = 0
	sm.useCases.AdvanceSession(ctx, session)
	return sm.sendSlotList(ctx, msg, result.Slots)
}

func (sm *StateMachine) handleSelectTime(
	ctx context.Context,
	msg IncomingMessage,
	session *Session,
	customer *customers.Customer,
) error {
	log.Printf("[scheduling] handleSelectTime | sessionID=%s interactiveID=%v",
		session.ID, msg.InteractiveID)

	if msg.InteractiveID == nil {
		log.Printf("[scheduling] no interactiveID in SELECT_TIME | sessionID=%s", session.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor selecciona una de las opciones de la lista 👆")
	}

	startsAt, resourceID, err := parseSlotID(*msg.InteractiveID)
	if err != nil {
		log.Printf("[scheduling] ERROR parsing slot id %q | sessionID=%s err=%v",
			*msg.InteractiveID, session.ID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Opción inválida. Por favor selecciona una de la lista 👆")
	}

	log.Printf("[scheduling] time slot selected | sessionID=%s startsAt=%s resourceID=%s",
		session.ID, startsAt.Format(time.RFC3339), resourceID)

	session.Data.StartsAt = &startsAt
	session.Data.ResourceID = &resourceID
	return sm.advanceToConfirmOrName(ctx, msg, session, customer)
}

func (sm *StateMachine) handleAwaitingName(ctx context.Context, msg IncomingMessage, session *Session, customer *customers.Customer) error {
	log.Printf("[scheduling] handleAwaitingName | sessionID=%s body=%q", session.ID, msg.Body)

	name := strings.TrimSpace(msg.Body)
	if len(name) < 2 {
		log.Printf("[scheduling] name too short | sessionID=%s name=%q", session.ID, name)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor dinos tu nombre para continuar 😊")
	}

	log.Printf("[scheduling] name captured | sessionID=%s name=%s", session.ID, name)
	sm.useCases.customers.UpdateName(ctx, customer.ID, name)
	customer.Name = &name
	session.Data.ConfirmedName = &name
	session.Step = StepConfirm
	sm.useCases.AdvanceSession(ctx, session)
	return sm.sendConfirmation(ctx, msg, session)
}

func (sm *StateMachine) handleConfirm(ctx context.Context, msg IncomingMessage, session *Session, customer *customers.Customer) error {
	log.Printf("[scheduling] handleConfirm | sessionID=%s interactiveID=%v",
		session.ID, msg.InteractiveID)

	if msg.InteractiveID == nil {
		log.Printf("[scheduling] no interactiveID in CONFIRM, resending summary | sessionID=%s", session.ID)
		return sm.sendConfirmation(ctx, msg, session)
	}

	switch *msg.InteractiveID {
	case "confirm_yes":
		log.Printf("[scheduling] customer confirmed appointment | sessionID=%s", session.ID)

		tenant, err := sm.tenantRepo.FindByID(ctx, session.TenantID)
		if err != nil {
			log.Printf("[scheduling] ERROR finding tenant in handleConfirm | tenantID=%s err=%v",
				session.TenantID, err)
			return fmt.Errorf("find tenant: %w", err)
		}

		appointment, err := sm.useCases.CreateAppointment(ctx, session, tenant.Timezone)
		if err != nil {
			log.Printf("[scheduling] ERROR creating appointment | sessionID=%s err=%v", session.ID, err)

			if errors.Is(err, apperrors.ErrOverlap) {
				log.Printf("[scheduling] overlap detected, fetching new suggestions | sessionID=%s", session.ID)
				svc, _ := sm.useCases.services.FindByID(ctx, *session.Data.ServiceID)
				suggestions, _ := sm.useCases.slotFinder.GetSuggestedSlots(
					ctx, *session.Data.ResourceID, *session.Data.StartsAt, svc)
				session.Step = StepSelectTime
				sm.useCases.AdvanceSession(ctx, session)
				errMsg := BuildErrorMessage(apperrors.ErrOverlap, "", suggestions)
				sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg)
				return sm.sendSlotList(ctx, msg, suggestions)
			}
			return err
		}

		log.Printf("[scheduling] appointment created | appointmentID=%s startsAt=%s",
			appointment.ID, appointment.StartsAt.Format(time.RFC3339))

		sm.sessions.Delete(ctx, session.ID)
		log.Printf("[scheduling] session deleted after booking | sessionID=%s", session.ID)
		return sm.sendAppointmentConfirmed(ctx, msg, appointment, customer, session)

	case "confirm_modify":
		log.Printf("[scheduling] customer wants to modify | sessionID=%s", session.ID)
		sm.sessions.Delete(ctx, session.ID)
		newSession, _ := sm.useCases.CreateSession(ctx, session.TenantID, session.WhatsappConfigID, customer.ID)
		return sm.sendServiceList(ctx, msg, newSession)

	case "confirm_cancel":
		log.Printf("[scheduling] customer cancelled flow | sessionID=%s", session.ID)
		sm.sessions.Delete(ctx, session.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Entendido, hemos cancelado el proceso 👋\nEscríbenos cuando quieras agendar.")
	}

	return sm.sendConfirmation(ctx, msg, session)
}

func (sm *StateMachine) handleMyAppointments(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	appointments, err := sm.useCases.appointments.FindByCustomer(ctx, msg.TenantID, customer.ID)
	if err != nil {
		return err
	}

	if len(appointments) == 0 {
		buttons := []whatsapp.Button{
			{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_schedule", Title: "📅 Agendar cita"}},
		}
		return sm.wa.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No tienes citas próximas agendadas 📭\n¿Deseas agendar una?", buttons)
	}

	text := "Tus próximas citas 📋\n\n"
	for _, a := range appointments {
		text += fmt.Sprintf("• %s – %s\n",
			a.StartsAt.Format("02/01 03:04 PM"),
			a.Status)
	}
	text += "\nPara cancelar una cita escribe *cancelar*."
	return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, text)
}

func (sm *StateMachine) handleCancelFlow(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	appointments, err := sm.useCases.appointments.FindByCustomer(ctx, msg.TenantID, customer.ID)
	if err != nil {
		return err
	}

	if len(appointments) == 0 {
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No tienes citas activas para cancelar 📭")
	}

	var rows []whatsapp.ListRow
	for _, a := range appointments {
		rows = append(rows, whatsapp.ListRow{
			ID:    "cancel_" + a.ID.String(),
			Title: a.StartsAt.Format("02/01 03:04 PM"),
		})
	}

	sections := []whatsapp.Section{
		{
			Title: "Elige una cita",
			Rows:  rows,
		},
	}
	return sm.wa.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Cuál cita deseas cancelar? 🗓️", sections)
}

func (sm *StateMachine) handleCancelConfirm(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	log.Printf("[scheduling] handleCancelConfirm | customerID=%s interactiveID=%s",
		customer.ID, *msg.InteractiveID)

	appointmentID, err := uuid.Parse(strings.TrimPrefix(*msg.InteractiveID, "cancel_"))
	if err != nil {
		log.Printf("[scheduling] ERROR parsing appointmentID from %q | err=%v",
			*msg.InteractiveID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	log.Printf("[scheduling] parsed appointmentID | id=%s", appointmentID)

	appointment, err := sm.useCases.appointments.FindByID(ctx, appointmentID, msg.TenantID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding appointment | id=%s err=%v", appointmentID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	if appointment.CustomerID != customer.ID {
		log.Printf("[scheduling] appointment does not belong to customer | appointmentID=%s customerID=%s",
			appointmentID, customer.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	svc, err := sm.useCases.services.FindByID(ctx, appointment.ServiceID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding service | serviceID=%s err=%v",
			appointment.ServiceID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	res, err := sm.useCases.resources.FindByID(ctx, appointment.ResourceID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding resource | resourceID=%s err=%v",
			appointment.ResourceID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	body := fmt.Sprintf(
		"¿Confirmas la cancelación de esta cita? 🗓️\n\n"+
			"✂️ %s con %s\n"+
			"📅 %s\n\n"+
			"Esta acción no se puede deshacer.",
		svc.Name, res.Name,
		appointment.StartsAt.Format("02/01/2006 03:04 PM"),
	)

	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_cancel_" + appointmentID.String(), Title: "✅ Sí, cancelar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_keep", Title: "🔙 No, mantener"}},
	}

	log.Printf("[scheduling] sending cancel confirmation | appointmentID=%s", appointmentID)
	return sm.wa.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

func (sm *StateMachine) handleCancelExecute(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	log.Printf("[scheduling] handleCancelExecute | customerID=%s interactiveID=%s",
		customer.ID, *msg.InteractiveID)

	appointmentID, err := uuid.Parse(strings.TrimPrefix(*msg.InteractiveID, "confirm_cancel_"))
	if err != nil {
		log.Printf("[scheduling] ERROR parsing appointmentID | interactiveID=%s err=%v",
			*msg.InteractiveID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error. Por favor intenta de nuevo.")
	}

	appointment, err := sm.useCases.appointments.FindByID(ctx, appointmentID, msg.TenantID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding appointment to cancel | id=%s err=%v", appointmentID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	if appointment.CustomerID != customer.ID {
		log.Printf("[scheduling] unauthorized cancel attempt | appointmentID=%s customerID=%s",
			appointmentID, customer.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"No encontramos esa cita. Por favor intenta de nuevo.")
	}

	if err := sm.useCases.appointments.UpdateStatus(
		ctx, appointmentID, "cancelled", "customer", "Cancelado por el cliente",
	); err != nil {
		log.Printf("[scheduling] ERROR cancelling appointment | id=%s err=%v", appointmentID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Ocurrió un error al cancelar. Por favor intenta de nuevo.")
	}

	log.Printf("[scheduling] appointment cancelled | id=%s", appointmentID)

	svc, err := sm.useCases.services.FindByID(ctx, appointment.ServiceID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding service after cancel | serviceID=%s err=%v",
			appointment.ServiceID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"✅ Tu cita ha sido cancelada.\n\n"+
				"Escríbenos cuando quieras agendar una nueva cita 👋")
	}

	res, err := sm.useCases.resources.FindByID(ctx, appointment.ResourceID)
	if err != nil {
		log.Printf("[scheduling] ERROR finding resource after cancel | resourceID=%s err=%v",
			appointment.ResourceID, err)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"✅ Tu cita ha sido cancelada.\n\n"+
				"Escríbenos cuando quieras agendar una nueva cita 👋")
	}

	return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		fmt.Sprintf(
			"✅ Tu cita ha sido cancelada.\n\n"+
				"✂️ %s con %s\n"+
				"📅 %s\n\n"+
				"Escríbenos cuando quieras agendar una nueva cita 👋",
			svc.Name, res.Name,
			appointment.StartsAt.Format("02/01/2006 03:04 PM"),
		),
	)
}

func (sm *StateMachine) sendServiceList(ctx context.Context, msg IncomingMessage, session *Session) error {
	services, err := sm.useCases.GetServices(ctx, session.TenantID)
	if err != nil {
		return err
	}

	var rows []whatsapp.ListRow
	for _, s := range services {
		rows = append(rows, whatsapp.ListRow{
			ID:          s.ID.String(),
			Title:       s.Name,
			Description: fmt.Sprintf("%d min · $%s", s.DurationMinutes, formatPrice(s.Price)),
		})
	}

	sections := []whatsapp.Section{{Title: "Servicios disponibles", Rows: rows}}
	return sm.wa.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Qué servicio deseas? ✂️", sections)
}

func (sm *StateMachine) sendResourceList(ctx context.Context, msg IncomingMessage, resources []resources.Resource) error {
	var rows []whatsapp.ListRow
	for _, r := range resources {
		rows = append(rows, whatsapp.ListRow{
			ID:    r.ID.String(),
			Title: r.Name,
		})
	}
	rows = append(rows, whatsapp.ListRow{
		ID:          "resource_any",
		Title:       "Sin preferencia",
		Description: "Te asignamos el primero disponible",
	})

	sections := []whatsapp.Section{{Title: "Elige tu barbero", Rows: rows}}
	return sm.wa.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Con quién deseas tu cita? 💈", sections)
}

func (sm *StateMachine) sendDatePrompt(ctx context.Context, msg IncomingMessage) error {
	body := "¿Para qué fecha y hora deseas tu cita? 📅\n\n" +
		"Escribe en este formato:\n*DD/MM HH:mm AM/PM*\n\n" +
		"Ejemplo: *15/03 09:00 AM*\n\n" +
		"Atendemos de lunes a sábado, 9:00 AM – 7:00 PM."
	return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body)
}

func (sm *StateMachine) sendSlotList(ctx context.Context, msg IncomingMessage, slots []TimeSlot) error {
	var rows []whatsapp.ListRow
	for _, s := range slots {
		rows = append(rows, whatsapp.ListRow{
			ID:          buildSlotID(s),
			Title:       s.StartsAt.Format("02/01 03:04 PM"),
			Description: s.ResourceName,
		})
	}

	sections := []whatsapp.Section{{Title: "Horarios disponibles", Rows: rows}}
	return sm.wa.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"Elige un horario disponible 🕐", sections)
}

func (sm *StateMachine) sendConfirmation(ctx context.Context, msg IncomingMessage, session *Session) error {
	svc, _ := sm.useCases.services.FindByID(ctx, *session.Data.ServiceID)
	res, _ := sm.useCases.resources.FindByID(ctx, *session.Data.ResourceID)

	clientName := "Cliente"
	if session.Data.ConfirmedName != nil {
		clientName = *session.Data.ConfirmedName
	}

	body := fmt.Sprintf(
		"Resumen de tu cita 📋\n\n"+
			"👤 Cliente:  %s\n"+
			"✂️ Servicio: %s (%d min)\n"+
			"💈 Barbero:  %s\n"+
			"📅 Fecha:    %s\n"+
			"💰 Precio:   $%s\n\n"+
			"¿Confirmamos?",
		clientName,
		svc.Name, svc.DurationMinutes,
		res.Name,
		session.Data.StartsAt.Format("02/01/2006 03:04 PM"),
		formatPrice(svc.Price),
	)

	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_yes", Title: "✅ Confirmar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_modify", Title: "✏️ Modificar"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "confirm_cancel", Title: "❌ Cancelar"}},
	}
	return sm.wa.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons)
}

func (sm *StateMachine) sendAppointmentConfirmed(ctx context.Context, msg IncomingMessage, a *Appointment, customer *customers.Customer, session *Session) error {
	svc, _ := sm.useCases.services.FindByID(ctx, a.ServiceID)
	res, _ := sm.useCases.resources.FindByID(ctx, a.ResourceID)
	tenant, _ := sm.tenantRepo.FindByID(ctx, session.TenantID)

	name := "Cliente"
	if customer.Name != nil {
		name = *customer.Name
	}

	body := fmt.Sprintf(
		"¡Listo, %s! 🎉 Tu cita está confirmada.\n\n"+
			"✂️ %s con %s\n"+
			"📅 %s\n"+
			"📍 %s\n\n"+
			"Te enviaremos un recordatorio 24 horas antes.\n"+
			"Si necesitas cancelar escríbenos aquí. ¡Hasta pronto! 👋",
		name,
		svc.Name, res.Name,
		a.StartsAt.Format("02/01/2006 03:04 PM"),
		tenant.Name,
	)
	return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body)
}

func (sm *StateMachine) advanceToConfirmOrName(
	ctx context.Context,
	msg IncomingMessage,
	session *Session,
	customer *customers.Customer,
) error {
	if customer.HasName() {
		log.Printf("[scheduling] customer already has name, going to CONFIRM | customerID=%s", customer.ID)
		name := *customer.Name
		session.Data.ConfirmedName = &name
		session.Step = StepConfirm
		sm.useCases.AdvanceSession(ctx, session)
		return sm.sendConfirmation(ctx, msg, session)
	}

	log.Printf("[scheduling] customer has no name, going to AWAITING_NAME | customerID=%s", customer.ID)
	session.Step = StepAwaitingName
	sm.useCases.AdvanceSession(ctx, session)
	return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"Antes de confirmar, ¿cuál es tu nombre? 😊")
}

func buildSlotID(slot TimeSlot) string {
	return fmt.Sprintf("slot_%s_%s", slot.StartsAt.UTC().Format(time.RFC3339), slot.ResourceID)
}

func parseSlotID(id string) (time.Time, uuid.UUID, error) {
	parts := strings.SplitN(id, "_", 3)
	if len(parts) != 3 {
		return time.Time{}, uuid.UUID{}, fmt.Errorf("invalid slot id")
	}
	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, uuid.UUID{}, err
	}
	rid, err := uuid.Parse(parts[2])
	if err != nil {
		return time.Time{}, uuid.UUID{}, err
	}
	return t, rid, nil
}

func formatPrice(p float64) string {
	return fmt.Sprintf("%.0f", p)
}
