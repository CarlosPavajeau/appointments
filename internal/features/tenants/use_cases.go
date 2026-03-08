package tenants

import (
	"context"
	"time"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
)

type UseCases struct {
	repo Repository
}

func NewUseCases(repo Repository) *UseCases {
	return &UseCases{repo: repo}
}

func (uc *UseCases) FindByID(ctx context.Context, id uuid.UUID) (*Tenant, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *UseCases) FindBySlug(ctx context.Context, slug string) (*Tenant, error) {
	return uc.repo.FindBySlug(ctx, slug)
}

func (uc *UseCases) Create(ctx context.Context, t *Tenant) error {
	return uc.repo.Create(ctx, t)
}

type ConnectWhatsappInput struct {
	TenantID           uuid.UUID
	WabaID             string
	PhoneNumberID      string
	DisplayPhoneNumber string
	AccessToken        string
	TokenExpiresAt     *time.Time
}

func (uc *UseCases) ConnectWhatsapp(ctx context.Context, input ConnectWhatsappInput) (*WhatsappConfig, error) {
	cfg := &WhatsappConfig{
		ID:                 uuid.New(),
		TenantID:           input.TenantID,
		WabaID:             input.WabaID,
		PhoneNumberID:      input.PhoneNumberID,
		DisplayPhoneNumber: input.DisplayPhoneNumber,
		AccessToken:        input.AccessToken,
		TokenExpiresAt:     input.TokenExpiresAt,
	}

	if err := uc.repo.CreateWhatsappConfig(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (uc *UseCases) VerifyWebhook(ctx context.Context, phoneNumberID string) error {
	cfg, _, err := uc.repo.FindWhatsappConfigByPhoneNumberID(ctx, phoneNumberID)
	if err != nil {
		return apperrors.ErrNotFound
	}

	cfg.VerifiedAt = new(time.Now())
	return uc.repo.UpdateWhatsappConfig(ctx, cfg)
}

func (uc *UseCases) UpdateSettings(ctx context.Context, tenantID uuid.UUID, settings TenantSettings) error {
	tenant, err := uc.repo.FindByID(ctx, tenantID)
	if err != nil {
		return err
	}
	tenant.Settings = settings
	return uc.repo.Update(ctx, tenant)
}

func (uc *UseCases) ResolveWebhook(ctx context.Context, phoneNumberID string) (*WhatsappConfig, *Tenant, error) {
	return uc.repo.FindWhatsappConfigByPhoneNumberID(ctx, phoneNumberID)
}

func (uc *UseCases) CreateWhatsappConfigPending(ctx context.Context, input CreateWhatsappConfigPendingInput) error {
	return uc.repo.CreateWhatsappConfigPending(ctx, input)
}

func (uc *UseCases) ActivateWhatsappConfig(ctx context.Context, input ActivateWhatsappConfigInput) error {
	return uc.repo.ActivateWhatsappConfig(ctx, input)
}

func (uc *UseCases) FindPendingActivations(ctx context.Context) ([]WhatsappConfig, error) {
	return uc.repo.FindPendingActivations(ctx)
}
