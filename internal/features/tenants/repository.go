package tenants

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*Tenant, error)
	Create(ctx context.Context, t *Tenant) error
	Update(ctx context.Context, t *Tenant) error
	IncrementAppointmentCount(ctx context.Context, id uuid.UUID) error
	ResetAppointmentCount(ctx context.Context, id uuid.UUID) error
	FindWhatsappConfig(ctx context.Context, tenantID uuid.UUID) (*WhatsappConfig, error)
	FindWhatsappConfigByPhoneNumberID(ctx context.Context, phoneNumberID string) (*WhatsappConfig, *Tenant, error)
	CreateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error
	UpdateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error
	CreateUser(ctx context.Context, u *TenantUser) error
	FindUserByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*TenantUser, error)
}

type tenantRow struct {
	ID                    uuid.UUID  `db:"id"`
	Name                  string     `db:"name"`
	Slug                  string     `db:"slug"`
	Timezone              string     `db:"timezone"`
	Currency              string     `db:"currency"`
	Plan                  string     `db:"plan"`
	PlanExpiresAt         *time.Time `db:"plan_expires_at"`
	AppointmentsThisMonth int        `db:"appointments_this_month"`
	MonthResetAt          time.Time  `db:"month_reset_at"`
	IsActive              bool       `db:"is_active"`
	Settings              []byte     `db:"settings"`
	CreatedAt             time.Time  `db:"created_at"`
	UpdatedAt             time.Time  `db:"updated_at"`
}

func (r tenantRow) toDomain() (*Tenant, error) {
	var settings TenantSettings
	if len(r.Settings) > 0 {
		if err := json.Unmarshal(r.Settings, &settings); err != nil {
			return nil, err
		}
	}
	return &Tenant{
		ID:                    r.ID,
		Name:                  r.Name,
		Slug:                  r.Slug,
		Timezone:              r.Timezone,
		Currency:              r.Currency,
		Plan:                  Plan(r.Plan),
		PlanExpiresAt:         r.PlanExpiresAt,
		AppointmentsThisMonth: r.AppointmentsThisMonth,
		MonthResetAt:          r.MonthResetAt,
		IsActive:              r.IsActive,
		Settings:              settings,
		CreatedAt:             r.CreatedAt,
		UpdatedAt:             r.UpdatedAt,
	}, nil
}

type whatsappConfigRow struct {
	ID                 uuid.UUID  `db:"id"`
	TenantID           uuid.UUID  `db:"tenant_id"`
	WabaID             string     `db:"waba_id"`
	PhoneNumberID      string     `db:"phone_number_id"`
	DisplayPhoneNumber string     `db:"display_phone_number"`
	AccessToken        string     `db:"access_token"` // encriptado en BD
	TokenExpiresAt     *time.Time `db:"token_expires_at"`
	IsActive           bool       `db:"is_active"`
	VerifiedAt         *time.Time `db:"verified_at"`
	CreatedAt          time.Time  `db:"created_at"`
	UpdatedAt          time.Time  `db:"updated_at"`
}
