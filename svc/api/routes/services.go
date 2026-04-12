package routes

import (
	"wappiz/internal/services/state_machine"
	"wappiz/pkg/crypto"
	"wappiz/pkg/db"
	"wappiz/pkg/mailer"
	"wappiz/pkg/runner"
	"wappiz/pkg/whatsapp"
)

// Services aggregates all dependencies required by API route handlers. It acts
// as a dependency injection container, allowing [Register] to wire up handlers
// without exposing individual dependencies throughout the codebase.
//
// This struct is constructed during server startup and passed to [Register].
// All fields except the optional configuration fields
// must be non-nil for the API to function correctly.
type Services struct {
	Database      db.Database                       // Primary database handle for all persistence operations.
	Mailer        mailer.Mailer                     // Email client for transactional messages.
	Whatsapp      whatsapp.Client                   // WhatsApp Business API client for messaging.
	StateMachine  state_machine.StateMachineService // Conversation state machine for WhatsApp booking flows.
	Runner        *runner.Runner                    // Runner for managing background tasks and graceful shutdown.
	AdminEmail    string                            // Destination address for internal admin notifications.
	AppSecret     string                            // Shared secret used to verify webhook signatures.
	Crypto        *crypto.Service                   // AES-GCM encrypt/decrypt for sensitive values at rest (e.g. access tokens).
}
