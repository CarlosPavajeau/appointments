package tenants

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wappiz/internal/platform/database"
	"wappiz/internal/shared/crypto"
	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*Tenant, error)
	Create(ctx context.Context, t *Tenant) error
	Update(ctx context.Context, t *Tenant) error
	LinkTenantUser(ctx context.Context, userID string, tenantID uuid.UUID) error
	IncrementAppointmentCount(ctx context.Context, id uuid.UUID) error
	ResetAppointmentCount(ctx context.Context, id uuid.UUID) error
	FindWhatsappConfig(ctx context.Context, tenantID uuid.UUID) (*WhatsappConfig, error)
	FindWhatsappConfigByPhoneNumberID(ctx context.Context, phoneNumberID string) (*WhatsappConfig, *Tenant, error)
	CreateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error
	UpdateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error
	CreateWhatsappConfigPending(ctx context.Context, input CreateWhatsappConfigPendingInput) error
	FindPendingActivations(ctx context.Context) ([]PendingActivation, error)
	FindPendingActivationByTenantID(ctx context.Context, tenantID uuid.UUID) (*PendingActivation, error)
	ActivateWhatsappConfig(ctx context.Context, input ActivateWhatsappConfigInput) error
}

type CreateWhatsappConfigPendingInput struct {
	TenantID     uuid.UUID
	ContactEmail string
	Notes        string
}

type ActivateWhatsappConfigInput struct {
	TenantID           uuid.UUID
	PhoneNumberID      string
	DisplayPhoneNumber string
	WABAID             string
	AccessToken        string
}

type pgRepository struct {
	db            *sqlx.DB
	encryptionKey []byte
}

func NewRepository(db *sqlx.DB, encryptionKey []byte) Repository {
	return &pgRepository{db: db, encryptionKey: encryptionKey}
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	var row tenantRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, name, slug, timezone, currency, plan, plan_expires_at,
		       appointments_this_month, month_reset_at, is_active, settings,
		       created_at, updated_at
		FROM tenants
		WHERE id = $1 AND is_active = true
	`, id)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	tenant, err := row.toDomain()
	if err != nil {
		return nil, err
	}

	if tenant.IsMonthExpired() {
		_ = r.ResetAppointmentCount(ctx, tenant.ID)
		tenant.AppointmentsThisMonth = 0
		tenant.MonthResetAt = firstDayOfNextMonth()
	}

	return tenant, nil
}

func (r *pgRepository) FindBySlug(ctx context.Context, slug string) (*Tenant, error) {
	var row tenantRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, name, slug, timezone, currency, plan, plan_expires_at,
		       appointments_this_month, month_reset_at, is_active, settings,
		       created_at, updated_at
		FROM tenants
		WHERE slug = $1 AND is_active = true
	`, slug)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain()
}

func (r *pgRepository) Create(ctx context.Context, t *Tenant) error {
	settings, err := jsonMarshal(t.Settings)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tenants
			(id, name, slug, timezone, currency, plan, appointments_this_month,
			 month_reset_at, is_active, settings)
		VALUES ($1,$2,$3,$4,$5,$6,0,$7,true,$8)
	`, t.ID, t.Name, t.Slug, t.Timezone, t.Currency, string(t.Plan),
		firstDayOfNextMonth(), settings)
	return err
}

func (r *pgRepository) LinkTenantUser(ctx context.Context, userID string, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenant_users (user_id, tenant_id, role)
		VALUES ($1, $2, 'admin')
		ON CONFLICT (user_id, tenant_id) DO NOTHING
	`, userID, tenantID)
	return err
}

func (r *pgRepository) Update(ctx context.Context, t *Tenant) error {
	settings, err := jsonMarshal(t.Settings)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE tenants
		SET name = $1, timezone = $2, settings = $3, updated_at = NOW()
		WHERE id = $4
	`, t.Name, t.Timezone, settings, t.ID)
	return err
}

func (r *pgRepository) IncrementAppointmentCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE tenants
		SET appointments_this_month = appointments_this_month + 1,
		    updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

func (r *pgRepository) ResetAppointmentCount(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE tenants
		SET appointments_this_month = 0,
		    month_reset_at = $1,
		    updated_at = NOW()
		WHERE id = $1
	`, firstDayOfNextMonth(), id)
	return err
}

func (r *pgRepository) FindWhatsappConfig(ctx context.Context, tenantID uuid.UUID) (*WhatsappConfig, error) {
	var row whatsappConfigRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, waba_id, phone_number_id, display_phone_number,
		       access_token, token_expires_at, is_active, verified_at,
		       created_at, updated_at
		FROM tenant_whatsapp_configs
		WHERE tenant_id = $1 AND is_active = true
		LIMIT 1
	`, tenantID)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r.rowToConfig(row)
}

func (r *pgRepository) FindWhatsappConfigByPhoneNumberID(ctx context.Context, phoneNumberID string) (*WhatsappConfig, *Tenant, error) {
	var row struct {
		whatsappConfigRow
		TenantName     string    `db:"tenant_name"`
		TenantSlug     string    `db:"tenant_slug"`
		TenantTimezone string    `db:"tenant_timezone"`
		TenantCurrency string    `db:"tenant_currency"`
		TenantPlan     string    `db:"tenant_plan"`
		TenantSettings []byte    `db:"tenant_settings"`
		TenantActive   bool      `db:"tenant_active"`
		MonthResetAt   time.Time `db:"month_reset_at"`
		ApptsMonth     int       `db:"appointments_this_month"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT
			twc.id, twc.tenant_id, twc.waba_id, twc.phone_number_id,
			twc.display_phone_number, twc.access_token, twc.token_expires_at,
			twc.is_active, twc.verified_at, twc.created_at, twc.updated_at,
			t.name   AS tenant_name,
			t.slug   AS tenant_slug,
			t.timezone AS tenant_timezone,
			t.currency AS tenant_currency,
			t.plan   AS tenant_plan,
			t.settings AS tenant_settings,
			t.is_active AS tenant_active,
			t.month_reset_at,
			t.appointments_this_month
		FROM tenant_whatsapp_configs twc
		JOIN tenants t ON t.id = twc.tenant_id
		WHERE twc.phone_number_id = $1
		  AND twc.is_active = true
		  AND t.is_active = true
		LIMIT 1
	`, phoneNumberID)

	if database.IsNotFound(err) {
		return nil, nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}

	cfg, err := r.rowToConfig(row.whatsappConfigRow)
	if err != nil {
		return nil, nil, err
	}

	var settings TenantSettings
	if len(row.TenantSettings) > 0 {
		json.Unmarshal(row.TenantSettings, &settings)
	}

	tenant := &Tenant{
		ID:                    row.TenantID,
		Name:                  row.TenantName,
		Slug:                  row.TenantSlug,
		Timezone:              row.TenantTimezone,
		Currency:              row.TenantCurrency,
		Plan:                  Plan(row.TenantPlan),
		AppointmentsThisMonth: row.ApptsMonth,
		MonthResetAt:          row.MonthResetAt,
		IsActive:              row.TenantActive,
		Settings:              settings,
	}

	return cfg, tenant, nil
}

func (r *pgRepository) CreateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error {
	encrypted, err := crypto.Encrypt(cfg.AccessToken, r.encryptionKey)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tenant_whatsapp_configs
			(id, tenant_id, waba_id, phone_number_id, display_phone_number,
			 access_token, token_expires_at, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,true)
	`, cfg.ID, cfg.TenantID, cfg.WabaID, cfg.PhoneNumberID,
		cfg.DisplayPhoneNumber, encrypted, cfg.TokenExpiresAt)
	return err
}

func (r *pgRepository) UpdateWhatsappConfig(ctx context.Context, cfg *WhatsappConfig) error {
	encrypted, err := crypto.Encrypt(cfg.AccessToken, r.encryptionKey)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE tenant_whatsapp_configs
		SET access_token = $1, token_expires_at = $2,
		    verified_at = $3, updated_at = NOW()
		WHERE id = $4
	`, encrypted, cfg.TokenExpiresAt, cfg.VerifiedAt, cfg.ID)
	return err
}

func (r *pgRepository) CreateWhatsappConfigPending(ctx context.Context, input CreateWhatsappConfigPendingInput) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenant_whatsapp_configs
			(id, tenant_id, activation_status, activation_requested_at, activation_contact_email, activation_notes)
		VALUES ($1, $2, 'pending', NOW(), $3, $4)
		ON CONFLICT (tenant_id) DO UPDATE
		SET
			activation_status        = 'pending',
			activation_requested_at  = NOW(),
			activation_contact_email = EXCLUDED.activation_contact_email,
			activation_notes         = EXCLUDED.activation_notes
	`, uuid.New(), input.TenantID, input.ContactEmail, input.Notes)
	return err
}

func (r *pgRepository) FindPendingActivations(ctx context.Context) ([]PendingActivation, error) {
	var rows []struct {
		TenantID              uuid.UUID  `db:"tenant_id"`
		TenantName            string     `db:"tenant_name"`
		ContactEmail          string     `db:"contact_email"`
		ActivationStatus      string     `db:"activation_status"`
		ActivationRequestedAt *time.Time `db:"activation_requested_at"`
		ActivationNotes       string     `db:"activation_notes"`
	}

	err := r.db.SelectContext(ctx, &rows, `
		SELECT
			wc.tenant_id,
			t.name                                          AS tenant_name,
			COALESCE(wc.activation_contact_email, '')       AS contact_email,
			wc.activation_status,
			wc.activation_requested_at,
			COALESCE(wc.activation_notes, '')               AS activation_notes
		FROM tenant_whatsapp_configs wc
		JOIN tenants t ON t.id = wc.tenant_id
		WHERE wc.activation_status = 'pending'
		ORDER BY wc.activation_requested_at ASC
	`)
	if err != nil {
		return nil, err
	}

	result := make([]PendingActivation, len(rows))
	for i, row := range rows {
		requestedAt := time.Time{}
		if row.ActivationRequestedAt != nil {
			requestedAt = *row.ActivationRequestedAt
		}
		result[i] = PendingActivation{
			TenantID:     row.TenantID,
			TenantName:   row.TenantName,
			ContactEmail: row.ContactEmail,
			Notes:        row.ActivationNotes,
			Status:       row.ActivationStatus,
			RequestedAt:  requestedAt,
		}
	}
	return result, nil
}

func (r *pgRepository) FindPendingActivationByTenantID(ctx context.Context, tenantID uuid.UUID) (*PendingActivation, error) {
	var row struct {
		TenantID              uuid.UUID  `db:"tenant_id"`
		TenantName            string     `db:"tenant_name"`
		ContactEmail          string     `db:"contact_email"`
		ActivationStatus      string     `db:"activation_status"`
		ActivationRequestedAt *time.Time `db:"activation_requested_at"`
		ActivationNotes       string     `db:"activation_notes"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT
			wc.tenant_id,
			t.name                                          AS tenant_name,
			COALESCE(wc.activation_contact_email, '')       AS contact_email,
			wc.activation_status,
			wc.activation_requested_at,
			COALESCE(wc.activation_notes, '')               AS activation_notes
		FROM tenant_whatsapp_configs wc
		JOIN tenants t ON t.id = wc.tenant_id
		WHERE wc.tenant_id = $1
		LIMIT 1
	`, tenantID)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	requestedAt := time.Time{}
	if row.ActivationRequestedAt != nil {
		requestedAt = *row.ActivationRequestedAt
	}
	return &PendingActivation{
		TenantID:     row.TenantID,
		TenantName:   row.TenantName,
		ContactEmail: row.ContactEmail,
		Notes:        row.ActivationNotes,
		Status:       row.ActivationStatus,
		RequestedAt:  requestedAt,
	}, nil
}

func (r *pgRepository) ActivateWhatsappConfig(ctx context.Context, input ActivateWhatsappConfigInput) error {
	encryptedToken, err := crypto.Encrypt(input.AccessToken, r.encryptionKey)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE tenant_whatsapp_configs
		SET
			waba_id               = $1,
			phone_number_id       = $2,
			display_phone_number  = $3,
			access_token          = $4,
			activation_status     = 'active',
			is_active             = true,
			verified_at           = NOW()
		WHERE tenant_id = $5
	`, input.WABAID, input.PhoneNumberID,
		input.DisplayPhoneNumber, encryptedToken,
		input.TenantID)
	return err
}

func (r *pgRepository) rowToConfig(row whatsappConfigRow) (*WhatsappConfig, error) {
	decrypted, err := crypto.Decrypt(row.AccessToken, r.encryptionKey)
	if err != nil {
		return nil, err
	}
	return &WhatsappConfig{
		ID:                 row.ID,
		TenantID:           row.TenantID,
		WabaID:             row.WabaID,
		PhoneNumberID:      row.PhoneNumberID,
		DisplayPhoneNumber: row.DisplayPhoneNumber,
		AccessToken:        decrypted, // decrypted in memory, never stored in plaintext
		TokenExpiresAt:     row.TokenExpiresAt,
		IsActive:           row.IsActive,
		VerifiedAt:         row.VerifiedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}, nil
}

func firstDayOfNextMonth() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
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
