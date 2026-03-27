package routes

import (
	"wappiz/internal/services/state_machine"
	"wappiz/pkg/db"
	"wappiz/pkg/mailer"
	"wappiz/pkg/whatsapp"
)

type Services struct {
	Database     db.Database
	Mailer       mailer.Mailer
	Whatsapp     whatsapp.Client
	StateMachine state_machine.StateMachineService
	AdminEmail   string
	AppSecret    string
}
