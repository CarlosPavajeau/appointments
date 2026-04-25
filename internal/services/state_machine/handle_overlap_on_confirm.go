package state_machine

import (
	"context"
	"wappiz/internal/services/slot_finder"
	"wappiz/pkg/db"
	apperrors "wappiz/pkg/errors"
	"wappiz/pkg/fault"
)

func (s *service) handleOverlapOnConfirm(
	ctx context.Context,
	msg IncomingMessage,
	session db.FindCustomerActiveConversationSessionRow,
	sessionData SessionData,
	svc db.FindServiceByIDRow,
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
		return fault.Wrap(err, fault.Internal("get suggested slots"))
	}

	filteredSuggestions := s.filterSlotsByCustomerAvailability(ctx, session.TenantID, session.CustomerID, suggestions)

	errMsg := buildErrorMessage(apperrors.ErrOverlap, "", filteredSuggestions)
	if err := s.whatsapp.SendText(ctx, msg.From, msg.PhoneNumberID, msg.AccessToken, errMsg); err != nil {
		return fault.Wrap(err, fault.Internal("send overlap message"))
	}

	if len(filteredSuggestions) == 0 {
		session.Step = string(StepSelectDate)
		if _, err = s.updateSession(ctx, session, sessionData); err != nil {
			return fault.Wrap(err, fault.Internal("update session"))
		}
		return nil
	}

	session.Step = string(StepSelectTime)
	if _, err = s.updateSession(ctx, session, sessionData); err != nil {
		return fault.Wrap(err, fault.Internal("update session"))
	}

	return s.sendSlotList(ctx, msg, filteredSuggestions)
}
