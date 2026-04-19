package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"

	"github.com/google/uuid"
)

func (s *service) handleSelectResource(ctx context.Context, msg IncomingMessage, session db.ConversationSession) error {
	interactiveID := msg.InteractiveID

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		logger.Error("[scheduling] failed to marshal session data on select resource step",
			"session_id", session.ID,
			"err", err)
		return s.sendServiceList(ctx, msg)
	}

	if interactiveID == nil {
		rsc, _ := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
			TenantID:  session.TenantID,
			ServiceID: *sessionData.ServiceID,
		})

		return s.sendResourceList(ctx, msg, rsc)
	}

	var resourceID *uuid.UUID
	if *interactiveID == "resource_any" {
		resourceID = nil
	} else {
		id, err := uuid.Parse(*interactiveID)
		if err != nil {
			rsc, _ := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
				TenantID:  session.TenantID,
				ServiceID: *sessionData.ServiceID,
			})

			return s.sendResourceList(ctx, msg, rsc)
		}

		resourceID = &id
	}

	sessionData.ResourceID = resourceID
	session.Step = string(StepSelectDate)

	if _, err := s.updateSession(ctx, session, sessionData); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return s.sendDatePrompt(ctx, msg)
}
