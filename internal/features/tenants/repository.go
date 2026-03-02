package tenants

import (
	"appointments/internal/platform/database"
	"appointments/internal/shared/crypto"
	apperrors "appointments/internal/shared/errors"
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
	FindUserByEmail(ctx context.Context, email string) (*TenantUser, error)
	FindUserByID(ctx context.Context, id uuid.UUID) (*TenantUser, error)
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

func (r *pgRepository) CreateUser(ctx context.Context, u *TenantUser) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenant_users (id, tenant_id, email, password_hash, role)
		VALUES ($1,$2,$3,$4,$5)
	`, u.ID, u.TenantID, u.Email, u.PasswordHash, u.Role)
	return err
}

func (r *pgRepository) FindUserByEmail(ctx context.Context, email string) (*TenantUser, error) {
	var row struct {
		ID           uuid.UUID `db:"id"`
		TenantID     uuid.UUID `db:"tenant_id"`
		Email        string    `db:"email"`
		PasswordHash string    `db:"password_hash"`
		Role         string    `db:"role"`
		CreatedAt    time.Time `db:"created_at"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, email, password_hash, role, created_at
		FROM tenant_users
		WHERE email = $2
	`, email)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &TenantUser{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *pgRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*TenantUser, error) {
	var row struct {
		ID           uuid.UUID `db:"id"`
		TenantID     uuid.UUID `db:"tenant_id"`
		Email        string    `db:"email"`
		PasswordHash string    `db:"password_hash"`
		Role         string    `db:"role"`
		CreatedAt    time.Time `db:"created_at"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, email, password_hash, role, created_at
		FROM tenant_users WHERE id = $1
	`, id)

	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return &TenantUser{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt,
	}, nil
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
