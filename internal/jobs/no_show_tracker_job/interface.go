package no_show_tracker_job

import "context"

type NoShowTrackerJob interface {
	Run(ctx context.Context)
}
