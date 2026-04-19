package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
)

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
