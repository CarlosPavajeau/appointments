package tenants

import (
	"context"
	"fmt"
	"regexp"
	"strings"
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

type RegisterTenantInput struct {
	Name     string
	Timezone string
	Email    string
	Password string
}

type RegisterTenantOutput struct {
	Tenant *Tenant
}

func (uc *UseCases) RegisterTenant(ctx context.Context, input RegisterTenantInput) (*RegisterTenantOutput, error) {
	slug := generateSlug(input.Name)

	if _, err := uc.repo.FindBySlug(ctx, slug); err == nil {
		slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:4])
	}

	tenant := &Tenant{
		ID:       uuid.New(),
		Name:     input.Name,
		Slug:     slug,
		Timezone: input.Timezone,
		Currency: "COP",
		Plan:     PlanFree,
	}

	if err := uc.repo.Create(ctx, tenant); err != nil {
		return nil, err
	}

	return &RegisterTenantOutput{Tenant: tenant}, nil
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

	now := time.Now()
	cfg.VerifiedAt = &now
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

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
	).Replace(slug)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	return slug
}
