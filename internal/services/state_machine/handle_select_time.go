package state_machine

import (
	"context"
	"encoding/json"
	"wappiz/pkg/db"
)

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
