package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const refreshTokenMaxAge = 7 * 24 * 3600 // 7 days in seconds

// setTokenCookies writes the access and refresh tokens as HttpOnly, Secure,
// SameSite=Strict cookies. The access token is scoped to /api/v1 and the
// refresh token is scoped to /api/v1/auth/refresh to limit exposure.
func setTokenCookies(c *gin.Context, pair *TokenPair) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("accessToken", pair.AccessToken, pair.ExpiresIn, "/api/v1", "", true, true)
	c.SetCookie("refreshToken", pair.RefreshToken, refreshTokenMaxAge, "/api/v1/auth/refresh", "", true, true)
}

// clearTokenCookies expires both auth cookies, used on logout.
func clearTokenCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("accessToken", "", -1, "/api/v1", "", true, true)
	c.SetCookie("refreshToken", "", -1, "/api/v1/auth/refresh", "", true, true)
}

// Handler handles HTTP requests for the /api/v1/auth endpoints.
type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

// RegisterRoutes mounts all auth routes on the provided engine.
//
//	POST /api/v1/auth/register — create tenant + admin user, returns tokens
//	POST /api/v1/auth/login    — authenticate, returns tokens
//	POST /api/v1/auth/refresh  — rotate refresh token, returns new tokens
//	POST /api/v1/auth/logout   — revoke refresh token
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	v1 := r.Group("/api/v1/auth")
	{
		v1.POST("/register", h.Register)
		v1.POST("/login", h.Login)
		v1.POST("/refresh", h.Refresh)
		v1.POST("/logout", h.Logout)
	}
}

type registerRequest struct {
	Name     string `json:"name"     binding:"required,min=2"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type loginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Register creates a new tenant and admin user, returning an initial token pair.
// POST /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, err := h.useCases.Register(c.Request.Context(), RegisterInput{
		Name:     req.Name,
		Timezone: "America/Bogota",
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	setTokenCookies(c, output.Tokens)
	c.JSON(http.StatusCreated, gin.H{
		"tenant": gin.H{
			"id":   output.Tenant.ID,
			"name": output.Tenant.Name,
			"slug": output.Tenant.Slug,
			"plan": output.Tenant.Plan,
		},
		"accessToken":  output.Tokens.AccessToken,
		"refreshToken": output.Tokens.RefreshToken,
		"expiresIn":    output.Tokens.ExpiresIn,
		"tokenType":    "Bearer",
	})
}

// Login authenticates a user and returns a token pair.
// POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, tenant, err := h.useCases.Login(c.Request.Context(), LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "login failed"})
		}
		return
	}

	setTokenCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{
		"tenant": gin.H{
			"id":   tenant.ID,
			"name": tenant.Name,
			"slug": tenant.Slug,
			"plan": tenant.Plan,
		},
		"accessToken":  pair.AccessToken,
		"refreshToken": pair.RefreshToken,
		"expiresIn":    pair.ExpiresIn,
		"tokenType":    "Bearer",
	})
}

// Refresh rotates the provided refresh token and issues a new token pair.
// POST /api/v1/auth/refresh
func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, tenant, err := h.useCases.RefreshTokens(c.Request.Context(), req.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrRefreshTokenReuse):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "token_reuse_detected",
				"hint":  "please log in again",
			})
		case errors.Is(err, ErrRefreshTokenExpired):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "refresh_token_expired",
				"hint":  "please log in again",
			})
		default:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_refresh_token"})
		}
		return
	}

	setTokenCookies(c, pair)
	c.JSON(http.StatusOK, gin.H{
		"tenant": gin.H{
			"id":   tenant.ID,
			"name": tenant.Name,
			"slug": tenant.Slug,
			"plan": tenant.Plan,
		},
		"accessToken":  pair.AccessToken,
		"refreshToken": pair.RefreshToken,
		"expiresIn":    pair.ExpiresIn,
		"tokenType":    "Bearer",
	})
}

// Logout revokes the provided refresh token. Always returns 200 to avoid
// leaking whether a token existed.
// POST /api/v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.useCases.Logout(c.Request.Context(), req.RefreshToken)
	clearTokenCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

