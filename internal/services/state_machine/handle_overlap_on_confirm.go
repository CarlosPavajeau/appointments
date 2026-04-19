package state_machine

import (
	"context"
	"fmt"
	"wappiz/internal/services/slot_finder"
	"wappiz/pkg/db"
	apperrors "wappiz/pkg/errors"
)

func (s *service) handleOverlapOnConfirm(
	ctx context.Context,
	msg IncomingMessage,
	session db.ConversationSession,
	sessionData SessionData,
	svc db.Service,
) error {
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

	filteredSuggestions := s.filterSlotsByCustomerAvailability(ctx, session.TenantID, session.CustomerID, suggestions)

	errMsg := buildErrorMessage(apperrors.ErrOverlap, "", filteredSuggestions)
	if err := s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg); err != nil {
		return err
	}

	if len(filteredSuggestions) == 0 {
		session.Step = string(StepSelectDate)
		if _, err = s.updateSession(ctx, session, sessionData); err != nil {
			return fmt.Errorf("update session: %w", err)
		}
		return nil
	}

	session.Step = string(StepSelectTime)
	if _, err = s.updateSession(ctx, session, sessionData); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.sendSlotList(ctx, msg, filteredSuggestions)
}
