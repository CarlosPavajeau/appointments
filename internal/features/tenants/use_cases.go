package tenants

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"wappiz/internal/platform/database"
	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
)

type UseCases struct {
	repo Repository
}

func NewUseCases(repo Repository) *UseCases {
	return &UseCases{repo: repo}
}

const slugAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const slugConstraint = "tenants_slug_key"
const slugMaxRetries = 5

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = slugAlphabet[rand.Intn(len(slugAlphabet))]
	}
	return string(b)
}

func (uc *UseCases) RegisterTenant(ctx context.Context, name, userID string) (*Tenant, error) {
	base := slugify(name)

	var tenant *Tenant
	for attempt := range slugMaxRetries {
		slug := base
		if attempt > 0 {
			slug = fmt.Sprintf("%s-%s", base, randomSuffix(5))
		}

		t := &Tenant{
			ID:       uuid.New(),
			Name:     name,
			Slug:     slug,
			Timezone: "America/Bogota",
			Currency: "COP",
			Plan:     PlanFree,
		}

		err := uc.repo.Create(ctx, t)
		if err == nil {
			tenant = t
			break
		}
		if !database.IsConstraintError(err, slugConstraint) {
			return nil, err
		}
	}

	if tenant == nil {
		return nil, errors.New("could not generate a unique slug after retries")
	}

	if err := uc.repo.LinkTenantUser(ctx, userID, tenant.ID); err != nil {
		return nil, err
	}

	return tenant, nil
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

func (uc *UseCases) FindPendingActivations(ctx context.Context) ([]PendingActivation, error) {
	return uc.repo.FindPendingActivations(ctx)
}

func (uc *UseCases) FindPendingActivationByTenantID(ctx context.Context, tenantID uuid.UUID) (*PendingActivation, error) {
	return uc.repo.FindPendingActivationByTenantID(ctx, tenantID)
}
