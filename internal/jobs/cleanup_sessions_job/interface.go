package cleanup_sessions_job

import "context"

type CleanupSessionsJob interface {
	Run(ctx context.Context)
}
