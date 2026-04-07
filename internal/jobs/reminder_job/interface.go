package reminder_job

import (
	"context"
	"wappiz/pkg/db"
	"wappiz/pkg/whatsapp"
)

type Config struct {
	DB            db.Database
	Whatsapp      whatsapp.Client
	EncryptionKey []byte
}

type ReminderJob interface {
	Run(ctx context.Context)
}
