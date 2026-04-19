package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
)

func (s *service) handleSelectService(ctx context.Context, msg IncomingMessage, session db.ConversationSession) error {
	svc, err := s.validateService(ctx, msg.TenantID, msg.InteractiveID)
	if err != nil {
		return s.sendServiceList(ctx, msg)
	}

	var sessionData SessionData
	if err := json.Unmarshal(session.Data, &sessionData); err != nil {
		logger.Error("[scheduling] failed to unmarshal session data on select service step",
			"session_id", session.ID,
			"err", err)
		return s.sendServiceList(ctx, msg)
	}

	sessionData.ServiceID = &svc.ID
	session.Step = string(StepSelectResource)
	session.ExpiresAt = time.Now().Add(sessionTTL)

	session, err = s.updateSession(ctx, session, sessionData)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	rsc, err := db.Query.FindResourcesByServiceID(ctx, s.db.Primary(), db.FindResourcesByServiceIDParams{
		TenantID:  session.TenantID,
		ServiceID: svc.ID,
	})

	if err != nil {
		return fmt.Errorf("find resources: %w", err)
	}

	if len(rsc) == 1 {
		sessionData.ResourceID = &rsc[0].ID
		session.Step = string(StepSelectDate)

		session, err = s.updateSession(ctx, session, sessionData)
		if err != nil {
			return fmt.Errorf("update session: %w", err)
		}

		return s.sendDatePrompt(ctx, msg)
	}

	return s.sendResourceList(ctx, msg, rsc)
}
