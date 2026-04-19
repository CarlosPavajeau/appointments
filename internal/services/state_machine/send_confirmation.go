package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"wappiz/pkg/date_formatter"
	"wappiz/pkg/db"
	"wappiz/pkg/whatsapp"
)

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
