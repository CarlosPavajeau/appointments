package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wappiz/internal/services/slot_finder"
	"wappiz/pkg/date_parser"
	"wappiz/pkg/db"
	apperrors "wappiz/pkg/errors"
	"wappiz/pkg/logger"
)

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

	endsAt := t.Add(time.Duration(svc.DurationMinutes) * time.Minute)

	// Check customer-level conflict for the requested time upfront so the
	// confirmation screen is never shown when the customer is already busy.
	customerConflict, err := s.hasCustomerOverlap(ctx, session.TenantID, session.CustomerID, t, endsAt)
	if err != nil {
		return nil, fmt.Errorf("check customer overlap: %w", err)
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

		if !customerConflict {
			for _, slot := range slots {
				if slot.StartsAt.Equal(t) {
					resourceID := *sessionData.ResourceID
					return &DateValidationResult{
						StartsAt:   t,
						ResourceID: &resourceID,
					}, nil
				}
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

		filtered := s.filterSlotsByCustomerAvailability(ctx, session.TenantID, session.CustomerID, suggestions)
		return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: filtered}, nil
	}

	rsc, err := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
		TenantID:  session.TenantID,
		ServiceID: *sessionData.ServiceID,
	})

	if err != nil {
		return nil, err
	}

	if !customerConflict {
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
	}

	// Find suggestions across all resources and filter out slots the customer
	// is already booked for.
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

	filtered := s.filterSlotsByCustomerAvailability(ctx, session.TenantID, session.CustomerID, allSuggestions)
	return &DateValidationResult{StartsAt: t, SlotTaken: true, Slots: filtered}, nil
}
