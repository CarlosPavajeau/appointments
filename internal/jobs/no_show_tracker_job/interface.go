package no_show_tracker_job

import (
	"context"
	"wappiz/pkg/crypto"
	"wappiz/pkg/db"
	"wappiz/pkg/whatsapp"
)

type Config struct {
	DB       db.Database
	Whatsapp whatsapp.Client
	Crypto   *crypto.Service
}

type NoShowTrackerJob interface {
	Run(ctx context.Context)
}
