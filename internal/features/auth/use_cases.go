package auth

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"appointments/internal/features/tenants"
	"appointments/internal/features/users"
	apperrors "appointments/internal/shared/errors"
	"appointments/internal/shared/jwt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TenantService defines the tenant operations needed by auth.
type TenantService interface {
	FindByID(ctx context.Context, id uuid.UUID) (*tenants.Tenant, error)
	FindBySlug(ctx context.Context, slug string) (*tenants.Tenant, error)
	Create(ctx context.Context, t *tenants.Tenant) error
}

// UserService defines the user operations needed by auth.
type UserService interface {
	Register(ctx context.Context, input users.RegisterUserInput) (*users.User, error)
	Get(ctx context.Context, id uuid.UUID) (*users.User, error)
	FindByEmail(ctx context.Context, email string) (*users.User, error)
}

// OnboardingService defines the onboarding operations needed by auth.
type OnboardingService interface {
	InitProgress(ctx context.Context, tenantID uuid.UUID) error
}

// UseCases handles all authentication and session lifecycle business logic.
type UseCases struct {
	tenantService    TenantService
	userService      UserService
	onboarding       OnboardingService
	refreshTokenRepo RefreshTokenRepository
}

func NewUseCases(
	tenantService TenantService,
	userService UserService,
	onboarding OnboardingService,
	refreshTokenRepo RefreshTokenRepository,
) *UseCases {
	return &UseCases{
		tenantService:    tenantService,
		userService:      userService,
		onboarding:       onboarding,
		refreshTokenRepo: refreshTokenRepo,
	}
}

// RegisterInput holds the data needed to register a new tenant and its admin user.
type RegisterInput struct {
	Name     string
	Timezone string
	Email    string
	Password string
}

// RegisterOutput holds the created entities and issued tokens.
type RegisterOutput struct {
	Tenant *tenants.Tenant
	User   *users.User
	Tokens *TokenPair
}

// Register creates a new tenant, its admin user, and returns an initial token pair.
func (uc *UseCases) Register(ctx context.Context, input RegisterInput) (*RegisterOutput, error) {
	slug := generateSlug(input.Name)
	if _, err := uc.tenantService.FindBySlug(ctx, slug); err == nil {
		slug = fmt.Sprintf("%s-%s", slug, uuid.New().String()[:4])
	}

	tenant := &tenants.Tenant{
		ID:       uuid.New(),
		Name:     input.Name,
		Slug:     slug,
		Timezone: input.Timezone,
		Currency: "COP",
		Plan:     tenants.PlanFree,
	}

	if err := uc.tenantService.Create(ctx, tenant); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	user, err := uc.userService.Register(ctx, users.RegisterUserInput{
		TenantID: tenant.ID,
		Email:    input.Email,
		Password: input.Password,
		Role:     "admin",
	})
	if err != nil {
		return nil, fmt.Errorf("register user: %w", err)
	}

	if err := uc.onboarding.InitProgress(ctx, tenant.ID); err != nil {
		return nil, fmt.Errorf("init onboarding: %w", err)
	}

	pair, err := uc.issueTokenPair(ctx, user, tenant, uuid.New())
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	return &RegisterOutput{Tenant: tenant, User: user, Tokens: pair}, nil
}

// LoginInput contains the credentials for authentication.
type LoginInput struct {
	Email    string
	Password string
}

// Login validates credentials and returns a token pair along with the tenant.
func (uc *UseCases) Login(ctx context.Context, input LoginInput) (*TokenPair, *tenants.Tenant, error) {
	user, err := uc.userService.FindByEmail(ctx, input.Email)
	if err != nil {
		log.Printf("login: user not found email=%s", input.Email)
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		log.Printf("login: wrong password user_id=%s email=%s", user.ID, input.Email)
		return nil, nil, ErrInvalidCredentials
	}

	tenant, err := uc.tenantService.FindByID(ctx, user.TenantID)
	if err != nil {
		log.Printf("login: tenant not found user_id=%s tenant_id=%s err=%v", user.ID, user.TenantID, err)
		return nil, nil, ErrInvalidCredentials
	}

	pair, err := uc.issueTokenPair(ctx, user, tenant, uuid.New())
	if err != nil {
		log.Printf("login: failed to issue tokens user_id=%s tenant_id=%s err=%v", user.ID, tenant.ID, err)
		return nil, nil, err
	}

	log.Printf("login: success user_id=%s tenant_id=%s", user.ID, tenant.ID)
	return pair, tenant, nil
}

// RefreshTokens rotates a refresh token and issues a new token pair.
// If a revoked token is presented, the entire token family is revoked (reuse detection).
func (uc *UseCases) RefreshTokens(ctx context.Context, plainToken string) (*TokenPair, *tenants.Tenant, error) {
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

	user, err := uc.userService.Get(ctx, rt.UserID)
	if err != nil {
		return nil, nil, err
	}

	tenant, err := uc.tenantService.FindByID(ctx, rt.TenantID)
	if err != nil {
		return nil, nil, err
	}

	pair, err := uc.issueTokenPair(ctx, user, tenant, rt.FamilyID)
	if err != nil {
		return nil, nil, err
	}

	return pair, tenant, nil
}

// Logout revokes the provided refresh token. It is idempotent: if the token is
// not found (already expired/revoked), it returns nil without error.
func (uc *UseCases) Logout(ctx context.Context, plainToken string) error {
	hash := jwt.HashToken(plainToken)
	rt, err := uc.refreshTokenRepo.FindByHash(ctx, hash)
	if err != nil {
		return nil
	}
	return uc.refreshTokenRepo.RevokeByID(ctx, rt.ID)
}

// issueTokenPair generates a new access/refresh token pair and persists the refresh token.
func (uc *UseCases) issueTokenPair(ctx context.Context, user *users.User, tenant *tenants.Tenant, familyID uuid.UUID) (*TokenPair, error) {
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

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n",
	).Replace(slug)
	slug = nonAlphanumeric.ReplaceAllString(slug, "-")
	return strings.Trim(slug, "-")
}
