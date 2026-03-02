package tenants

import (
	"appointments/internal/shared/jwt"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UseCases struct {
	repo             Repository
	refreshTokenRepo RefreshTokenRepository
}

func NewUseCases(repo Repository, refreshTokenRepo RefreshTokenRepository) *UseCases {
	return &UseCases{repo: repo, refreshTokenRepo: refreshTokenRepo}
}

type RegisterTenantInput struct {
	Name     string
	Timezone string
	Email    string
	Password string
}

type RegisterTenantOutput struct {
	Tenant *Tenant
	User   *TenantUser
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

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &TenantUser{
		ID:           uuid.New(),
		TenantID:     tenant.ID,
		Email:        input.Email,
		PasswordHash: string(hash),
		Role:         "admin",
	}

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return &RegisterTenantOutput{Tenant: tenant, User: user}, nil
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

type LoginInput struct {
	Email    string
	Password string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func (uc *UseCases) Login(ctx context.Context, input LoginInput) (*TokenPair, *Tenant, error) {
	user, err := uc.repo.FindUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, nil, apperrors.ErrNotFound
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, nil, apperrors.ErrNotFound
	}

	tenant, err := uc.repo.FindByID(ctx, user.TenantID)
	if err != nil {
		return nil, nil, apperrors.ErrNotFound
	}

	pair, err := uc.issueTokenPair(ctx, user, tenant, uuid.New())
	if err != nil {
		return nil, nil, err
	}

	return pair, tenant, nil
}

func (uc *UseCases) RefreshTokens(ctx context.Context, plainToken string) (*TokenPair, *Tenant, error) {
	hash := jwt.HashToken(plainToken)

	rt, err := uc.refreshTokenRepo.FindByHash(ctx, hash)
	if err != nil {
		return nil, nil, apperrors.ErrNotFound
	}

	if rt.IsRevoked() {
		uc.refreshTokenRepo.RevokeFamily(ctx, rt.FamilyID)
		return nil, nil, ErrRefreshTokenReuse
	}

	if rt.IsExpired() {
		uc.refreshTokenRepo.RevokeByID(ctx, rt.ID)
		return nil, nil, ErrRefreshTokenExpired
	}

	if err := uc.refreshTokenRepo.RevokeByID(ctx, rt.ID); err != nil {
		return nil, nil, err
	}

	user, err := uc.repo.FindUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, nil, err
	}

	tenant, err := uc.repo.FindByID(ctx, rt.TenantID)
	if err != nil {
		return nil, nil, err
	}

	pair, err := uc.issueTokenPair(ctx, user, tenant, rt.FamilyID)
	if err != nil {
		return nil, nil, err
	}

	return pair, tenant, nil
}

func (uc *UseCases) Logout(ctx context.Context, plainToken string) error {
	hash := jwt.HashToken(plainToken)
	rt, err := uc.refreshTokenRepo.FindByHash(ctx, hash)
	if err != nil {
		return nil
	}
	return uc.refreshTokenRepo.RevokeByID(ctx, rt.ID)
}

func (uc *UseCases) issueTokenPair(ctx context.Context, user *TenantUser, tenant *Tenant, familyID uuid.UUID) (*TokenPair, error) {
	accessToken, err := jwt.GenerateAccessToken(user.ID, tenant.ID, user.Role)
	if err != nil {
		return nil, err
	}

	plain, hash, err := jwt.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	rt := &RefreshToken{
		ID:        uuid.New(),
		TenantID:  tenant.ID,
		UserID:    user.ID,
		TokenHash: hash,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(jwt.RefreshTokenDuration),
	}

	if err := uc.refreshTokenRepo.Create(ctx, rt); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: plain,
		ExpiresIn:    int(jwt.AccessTokenDuration.Seconds()),
	}, nil
}

var (
	ErrRefreshTokenReuse   = errors.New("refresh token reuse detected")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

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
