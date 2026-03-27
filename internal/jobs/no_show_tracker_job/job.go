package no_show_tracker_job

import (
	"context"
	"database/sql"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"

	"github.com/google/uuid"
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
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	logger.Info("[no_show_tracker] started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("[no_show_tracker] stopped")
			return
		case <-ticker.C:
			if err := j.process(ctx); err != nil {
				logger.Error("[no_show_tracker] failed to process job %v", err)
			}
		}
	}
}

func (j *job) process(ctx context.Context) error {
	unattended, err := db.Query.FindUnattendedAppointments(ctx, j.db.Primary())
	if err != nil {
		logger.Warn("[no_show_tracker] failed to find unattended appointments: %v", err)
		return err
	}

	affected := make(map[uuid.UUID]uuid.UUID)
	for _, a := range unattended {
		if err := db.Query.UpdateAppointment(ctx, j.db.Primary(), db.UpdateAppointmentParams{
			Status:       db.AppointmentStatusNoShow,
			CancelledBy:  sql.NullString{},
			CancelReason: sql.NullString{},
			CompletedAt:  sql.NullTime{},
			ID:           a.ID,
		}); err != nil {
			logger.Warn("[no_show_tracker] failed to update appointment: %v", err)
			continue
		}

		if err := db.Query.InsertAppointmentStatusHistory(ctx, j.db.Primary(), db.InsertAppointmentStatusHistoryParams{
			ID:            uuid.New(),
			AppointmentID: a.ID,
			FromStatus:    a.Status,
			ToStatus:      db.AppointmentStatusNoShow,
			ChangedBy:     sql.NullString{},
			ChangedByRole: sql.NullString{String: "system", Valid: true},
			Reason:        sql.NullString{String: "Auto-detected: customer did not check in", Valid: true},
		}); err != nil {
			logger.Warn("[no_show_tracker] failed to insert appointment history: %v", err)
			continue
		}

		affected[a.CustomerID] = a.TenantID
	}

	processed := make(map[uuid.UUID]bool)
	for customerID := range affected {
		// TODO: evaluate customer
		processed[customerID] = true
	}

	recentlyCancelled, err := db.Query.FindRecentlyCancelledAppointments(ctx, j.db.Primary())
	if err != nil {
		logger.Warn("[no_show_tracker] failed to find recently cancelled appointments: %v", err)
		return err
	}

	for _, a := range recentlyCancelled {
		if processed[a.CustomerID] {
			continue
		}

		// TODO: Process recently cancelled
	}

	return nil
}
