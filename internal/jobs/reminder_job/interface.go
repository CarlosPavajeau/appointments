package reminder_job

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

type ReminderJob interface {
	Run(ctx context.Context)
}
