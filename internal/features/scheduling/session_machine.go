package scheduling

import (
	"appointments/internal/features/customers"
	"appointments/internal/features/resources"
	"appointments/internal/features/tenants"
	"context"
	"errors"
	"fmt"
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
	customer, err := sm.useCases.ResolveCustomer(ctx, msg.TenantID, msg.From)
	if err != nil {
		return err
	}
	if customer.IsBlocked {
		return nil
	}

	session, err := sm.sessions.FindActive(ctx, msg.TenantID, customer.ID)
	if err != nil && !errors.Is(err, apperrors.ErrSessionNotFound) {
		return err
	}

	if session == nil {
		return sm.handleEntry(ctx, msg, customer)
	}

	switch session.Step {
	case StepSelectService:
		return sm.handleSelectService(ctx, msg, session)
	case StepSelectResource:
		return sm.handleSelectResource(ctx, msg, session)
	case StepSelectDate:
		return sm.handleSelectDate(ctx, msg, session)
	case StepSelectTime:
		return sm.handleSelectTime(ctx, msg, session)
	case StepAwaitingName:
		return sm.handleAwaitingName(ctx, msg, session, customer)
	case StepConfirm:
		return sm.handleConfirm(ctx, msg, session, customer)
	default:
		sm.sessions.Delete(ctx, session.ID)
		return sm.handleEntry(ctx, msg, customer)
	}
}

func (sm *StateMachine) handleEntry(ctx context.Context, msg IncomingMessage, customer *customers.Customer) error {
	tenant, err := sm.tenantRepo.FindByID(ctx, msg.TenantID)
	if err != nil {
		return err
	}

	body := fmt.Sprintf("👋 ¡Hola! Bienvenido a *%s*\n\n¿Qué deseas hacer?", tenant.Name)
	buttons := []whatsapp.Button{
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_schedule", Title: "📅 Agendar cita"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_my_appointments", Title: "📋 Mis citas"}},
		{Type: "reply", Reply: whatsapp.ButtonReply{ID: "action_cancel", Title: "❌ Cancelar cita"}},
	}
	if err := sm.wa.SendButtons(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, body, buttons); err != nil {
		return err
	}

	if msg.InteractiveID == nil {
		return nil
	}

	switch *msg.InteractiveID {
	case "action_schedule":
		session, err := sm.useCases.CreateSession(ctx, msg.TenantID, msg.WhatsappConfigID, customer.ID)
		if err != nil {
			return err
		}
		return sm.sendServiceList(ctx, msg, session)

	case "action_my_appointments":
		return sm.handleMyAppointments(ctx, msg, customer)

	case "action_cancel":
		return sm.handleCancelFlow(ctx, msg, customer)
	}

	return nil
}

func (sm *StateMachine) handleSelectService(ctx context.Context, msg IncomingMessage, session *Session) error {
	svc, err := sm.useCases.ValidateService(ctx, session.TenantID, msg.InteractiveID)
	if err != nil {
		return sm.sendServiceList(ctx, msg, session)
	}

	session.Data.ServiceID = &svc.ID
	session.Step = StepSelectResource
	if err := sm.useCases.AdvanceSession(ctx, session); err != nil {
		return err
	}

	resources, err := sm.useCases.GetResourcesForService(ctx, session.TenantID, svc.ID)
	if err != nil {
		return err
	}

	// Skip if only one resource available
	if len(resources) == 1 {
		session.Data.ResourceID = &resources[0].ID
		session.Step = StepSelectDate
		sm.useCases.AdvanceSession(ctx, session)
		return sm.sendDatePrompt(ctx, msg)
	}

	return sm.sendResourceList(ctx, msg, resources)
}

func (sm *StateMachine) handleSelectResource(ctx context.Context, msg IncomingMessage, session *Session) error {
	if msg.InteractiveID == nil {
		resources, _ := sm.useCases.GetResourcesForService(ctx, session.TenantID, *session.Data.ServiceID)
		return sm.sendResourceList(ctx, msg, resources)
	}

	var resourceID *uuid.UUID
	if *msg.InteractiveID == "resource_any" {
		resourceID = nil
	} else {
		id, err := uuid.Parse(*msg.InteractiveID)
		if err != nil {
			resources, _ := sm.useCases.GetResourcesForService(ctx, session.TenantID, *session.Data.ServiceID)
			return sm.sendResourceList(ctx, msg, resources)
		}
		resourceID = &id
	}

	session.Data.ResourceID = resourceID
	session.Step = StepSelectDate
	if err := sm.useCases.AdvanceSession(ctx, session); err != nil {
		return err
	}

	return sm.sendDatePrompt(ctx, msg)
}

func (sm *StateMachine) handleSelectDate(ctx context.Context, msg IncomingMessage, session *Session) error {
	if session.Data.DateAttempts >= maxDateAttempts {
		sm.sessions.Delete(ctx, session.ID)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Parece que estás teniendo problemas para agendar 😅\n"+
				"Escríbenos cuando quieras e intentamos de nuevo.\n\n"+
				"Escribe *hola* para comenzar.")
	}

	tenant, err := sm.tenantRepo.FindByID(ctx, session.TenantID)
	if err != nil {
		return err
	}

	result, err := sm.useCases.ValidateAndFindSlots(ctx, msg.Body, tenant.Timezone, session)
	if err != nil {
		session.Data.DateAttempts++
		sm.useCases.AdvanceSession(ctx, session)
		errMsg := BuildErrorMessage(err, msg.Body, nil)
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg)
	}

	if !result.SlotTaken {
		session.Data.StartsAt = &result.StartsAt
		session.Data.DateAttempts = 0
		session.Step = StepConfirm
		sm.useCases.AdvanceSession(ctx, session)
		return sm.sendConfirmation(ctx, msg, session)
	}

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

func (sm *StateMachine) handleSelectTime(ctx context.Context, msg IncomingMessage, session *Session) error {
	if msg.InteractiveID == nil {
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor selecciona una de las opciones de la lista 👆")
	}

	startsAt, resourceID, err := parseSlotID(*msg.InteractiveID)
	if err != nil {
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Opción inválida. Por favor selecciona una de la lista 👆")
	}

	session.Data.StartsAt = &startsAt
	session.Data.ResourceID = &resourceID
	session.Step = StepConfirm
	sm.useCases.AdvanceSession(ctx, session)
	return sm.sendConfirmation(ctx, msg, session)
}

func (sm *StateMachine) handleAwaitingName(ctx context.Context, msg IncomingMessage, session *Session, customer *customers.Customer) error {
	name := msg.Body
	if len(name) < 2 {
		return sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
			"Por favor dinos tu nombre para continuar 😊")
	}

	sm.useCases.customers.UpdateName(ctx, customer.ID, name)
	customer.Name = &name

	session.Data.ConfirmedName = &name
	session.Step = StepConfirm
	sm.useCases.AdvanceSession(ctx, session)
	return sm.sendConfirmation(ctx, msg, session)
}

func (sm *StateMachine) handleConfirm(ctx context.Context, msg IncomingMessage, session *Session, customer *customers.Customer) error {
	if msg.InteractiveID == nil {
		return sm.sendConfirmation(ctx, msg, session)
	}

	switch *msg.InteractiveID {
	case "confirm_yes":
		tenant, err := sm.tenantRepo.FindByID(ctx, session.TenantID)
		if err != nil {
			return err
		}

		appointment, err := sm.useCases.CreateAppointment(ctx, session, tenant.Timezone)
		if err != nil {
			if errors.Is(err, apperrors.ErrOverlap) {
				service, _ := sm.useCases.services.FindByID(ctx, *session.Data.ServiceID)
				suggestions, _ := sm.useCases.slotFinder.GetSuggestedSlots(
					ctx, *session.Data.ResourceID, *session.Data.StartsAt, service)
				session.Step = StepSelectTime
				sm.useCases.AdvanceSession(ctx, session)
				errMsg := BuildErrorMessage(apperrors.ErrOverlap, "", suggestions)
				sm.wa.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg)
				return sm.sendSlotList(ctx, msg, suggestions)
			}
			return err
		}

		sm.sessions.Delete(ctx, session.ID)
		return sm.sendAppointmentConfirmed(ctx, msg, appointment, customer, session)

	case "confirm_modify":
		sm.sessions.Delete(ctx, session.ID)
		newSession, _ := sm.useCases.CreateSession(ctx, session.TenantID, session.WhatsappConfigID, customer.ID)
		return sm.sendServiceList(ctx, msg, newSession)

	case "confirm_cancel":
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

	sections := []whatsapp.Section{{Title: "Selecciona la cita a cancelar", Rows: rows}}
	return sm.wa.SendList(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"¿Cuál cita deseas cancelar?", sections)
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
