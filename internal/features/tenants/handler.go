package tenants

import (
	"appointments/internal/shared/jwt"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	public := r.Group("/api/v1")
	{
		public.POST("/tenants/register", h.Register)
		public.POST("/tenants/login", h.Login)
		public.POST("/auth/refresh", h.Refresh)
		public.POST("/auth/logout", h.Logout)
	}

	protected := r.Group("/api/v1/tenants")
	protected.Use(jwt.AuthMiddleware())
	{
		protected.GET("/me", h.GetMe)
		protected.PUT("/settings", h.UpdateSettings)
		protected.POST("/whatsapp", h.ConnectWhatsapp)
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

type connectWhatsappRequest struct {
	WabaID             string `json:"waba_id"              binding:"required"`
	PhoneNumberID      string `json:"phone_number_id"      binding:"required"`
	DisplayPhoneNumber string `json:"display_phone_number" binding:"required"`
	AccessToken        string `json:"access_token"         binding:"required"`
}

type updateSettingsRequest struct {
	WelcomeMessage  string `json:"welcome_message"`
	BotName         string `json:"bot_name"`
	CancellationMsg string `json:"cancellation_message"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, err := h.useCases.RegisterTenant(c.Request.Context(), RegisterTenantInput{
		Name:     req.Name,
		Timezone: "America/Bogota",
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	pair, err := h.useCases.issueTokenPair(
		c.Request.Context(),
		output.User,
		output.Tenant,
		uuid.New(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"tenant": gin.H{
			"id":   output.Tenant.ID,
			"name": output.Tenant.Name,
			"slug": output.Tenant.Slug,
			"plan": output.Tenant.Plan,
		},
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

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

	c.JSON(http.StatusOK, gin.H{
		"tenant": gin.H{
			"id":   tenant.ID,
			"name": tenant.Name,
			"slug": tenant.Slug,
			"plan": tenant.Plan,
		},
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

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

	c.JSON(http.StatusOK, gin.H{
		"tenant": gin.H{
			"id":   tenant.ID,
			"name": tenant.Name,
			"slug": tenant.Slug,
			"plan": tenant.Plan,
		},
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.useCases.Logout(c.Request.Context(), req.RefreshToken)
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) GetMe(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)
	tenant, err := h.useCases.repo.FindByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      tenant.ID,
		"name":                    tenant.Name,
		"slug":                    tenant.Slug,
		"timezone":                tenant.Timezone,
		"plan":                    tenant.Plan,
		"appointments_this_month": tenant.AppointmentsThisMonth,
		"settings":                tenant.Settings,
	})
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	err := h.useCases.UpdateSettings(c.Request.Context(), tenantID, TenantSettings{
		WelcomeMessage:  req.WelcomeMessage,
		BotName:         req.BotName,
		CancellationMsg: req.CancellationMsg,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}

func (h *Handler) ConnectWhatsapp(c *gin.Context) {
	var req connectWhatsappRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	cfg, err := h.useCases.ConnectWhatsapp(c.Request.Context(), ConnectWhatsappInput{
		TenantID:           tenantID,
		WabaID:             req.WabaID,
		PhoneNumberID:      req.PhoneNumberID,
		DisplayPhoneNumber: req.DisplayPhoneNumber,
		AccessToken:        req.AccessToken,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connection failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"phone_number_id":      cfg.PhoneNumberID,
		"display_phone_number": cfg.DisplayPhoneNumber,
		"is_active":            cfg.IsActive,
	})
}
