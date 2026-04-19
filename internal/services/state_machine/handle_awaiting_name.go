package state_machine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"wappiz/pkg/db"
)

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
