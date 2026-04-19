package state_machine

import (
	"context"
	"encoding/json"
	"fmt"
	"wappiz/pkg/db"
)

func (s *service) updateSession(ctx context.Context, session db.ConversationSession, sessionData SessionData) (db.ConversationSession, error) {
	updatedData, err := json.Marshal(sessionData)
	if err != nil {
		return session, fmt.Errorf("marshal session data: %w", err)
	}

	session.Data = updatedData

	if err := db.Query.UpdateConversationSession(ctx, s.db.Primary(), db.UpdateConversationSessionParams{
		Step:      session.Step,
		Data:      session.Data,
		ExpiresAt: session.ExpiresAt,
		ID:        session.ID,
	}); err != nil {
		return session, fmt.Errorf("update session: %w", err)
	}

	return session, nil
}
