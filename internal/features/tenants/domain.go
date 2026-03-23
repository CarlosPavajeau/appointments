package tenants

import (
	"time"

	"github.com/google/uuid"
)

type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)

type Tenant struct {
	ID                    uuid.UUID
	Name                  string
	Slug                  string
	Timezone              string
	Currency              string
	Plan                  Plan
	PlanExpiresAt         *time.Time
	AppointmentsThisMonth int
	MonthResetAt          time.Time
	IsActive              bool
	Settings              TenantSettings
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type TenantSettings struct {
	WelcomeMessage           string `json:"welcomeMessage,omitempty"`
	BotName                  string `json:"botName,omitempty"`
	CancellationMsg          string `json:"cancellationMessage,omitempty"`
	ContactEmail             string `json:"contactEmail,omitempty"`
	LateCancelHours          int    `json:"lateCancelHours"`            // default: 2
	AutoBlockAfterNoShows    int    `json:"autoBlockAfterNoShows"`      // default: 3
	AutoBlockAfterLateCancel int    `json:"autoBlockAfterLateCancel"`   // default: 3
	SendWarningBeforeBlock   bool   `json:"sendWarningBeforeBlock"`
}

type WhatsappConfig struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	WabaID             string
	PhoneNumberID      string
	DisplayPhoneNumber string
	AccessToken        string // memory decrypted, never stored in plaintext
	TokenExpiresAt     *time.Time
	IsActive           bool
	ActivationStatus   string
	VerifiedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type PendingActivation struct {
	TenantID     uuid.UUID
	TenantName   string
	ContactEmail string
	Notes        string
	Status       string
	RequestedAt  time.Time
}

type TenantUser struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

var FreemiumLimits = map[Plan]int{
	PlanFree: 50, // 50 appointments per month
	PlanPro:  -1, // -1 = unlimited
}

func (t *Tenant) HasReachedAppointmentLimit() bool {
	limit := FreemiumLimits[t.Plan]
	if limit == -1 {
		return false
	}
	return t.AppointmentsThisMonth >= limit
}

func (t *Tenant) IsMonthExpired() bool {
	return time.Now().After(t.MonthResetAt)
}

func (t *Tenant) WelcomeMessage() string {
	if t.Settings.WelcomeMessage != "" {
		return t.Settings.WelcomeMessage
	}
	return "¡Hola! Bienvenido a *" + t.Name + "*"
}

func (t *Tenant) BotName() string {
	if t.Settings.BotName != "" {
		return t.Settings.BotName
	}
	return t.Name
}

func (t *Tenant) CancellationMessage() string {
	if t.Settings.CancellationMsg != "" {
		return t.Settings.CancellationMsg
	}
	return "Escríbenos cuando quieras agendar una nueva cita 👋"
}
