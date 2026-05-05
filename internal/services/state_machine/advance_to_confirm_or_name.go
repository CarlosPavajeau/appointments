package state_machine

import (
	"context"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
)

func (s *service) advanceToConfirmOrName(ctx context.Context, msg IncomingMessage, session db.ConversationSession, sessionData SessionData, customer db.FindCustomerByPhoneNumberRow) error {
	var err error
	if customer.Name.Valid {
		sessionData.ConfirmedName = new(customer.Name.String)
		session.Step = string(StepConfirm)

		session, err = s.updateSession(ctx, session, sessionData)
		if err != nil {
			return fault.Wrap(err, fault.Internal("update session"))
		}

		return s.sendConfirmation(ctx, msg, session)
	}

	session.Step = string(StepAwaitingName)
	if _, err = s.updateSession(ctx, session, sessionData); err != nil {
		return fault.Wrap(err, fault.Internal("update session"))
	}

	return s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken,
		"Antes de confirmar, ¿cuál es tu nombre? 😊")
}
