package cleanup_sessions_job

import (
	"context"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
)

type job struct {
	db db.Database
}

func New(db db.Database) *job {
	return &job{
		db: db,
	}
}

func (j *job) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	logger.Info("[cleanup_sessions_job] started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("[cleanup_sessions_job] stopped")
			return
		case <-ticker.C:
			if err := db.Query.DeleteExpiredConversationSessions(ctx, j.db.Primary()); err != nil {
				logger.Warn("[cleanup_sessions_job] failed to delete expired sessions: %v", err)
			}
		}
	}
}
