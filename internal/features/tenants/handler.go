package tenants

import (
	"context"
	"net/http"

	"wappiz/internal/shared/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// OnboardingInitializer is the subset of the onboarding use cases needed here.
type OnboardingInitializer interface {
	InitProgress(ctx context.Context, tenantID uuid.UUID) error
}

type Handler struct {
	useCases   *UseCases
	onboarding OnboardingInitializer
}

func NewHandler(uc *UseCases, ob OnboardingInitializer) *Handler {
	return &Handler{useCases: uc, onboarding: ob}
}

// RegisterRoutes mounts protected tenant management endpoints.
//
//	POST /api/v1/tenants           — create a new tenant
//	GET  /api/v1/tenants/me        — get current tenant info
//	GET  /api/v1/tenants/by-user   — get tenant by authenticated user ID
//	PUT  /api/v1/tenants/settings  — update tenant settings
//	POST /api/v1/tenants/whatsapp  — connect WhatsApp config
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	protected := r.Group("/api/v1/tenants")
	protected.Use(jwt.AuthMiddleware())
	{
		protected.POST("", h.CreateTenant)
		protected.GET("/me", h.GetTenant)
		protected.GET("/by-user", h.GetTenantByUser)
		protected.PUT("/settings", h.UpdateSettings)
		protected.POST("/whatsapp", h.ConnectWhatsapp)
	}
}

func tenantResponse(t *Tenant) gin.H {
	return gin.H{
		"id":       t.ID,
		"name":     t.Name,
		"slug":     t.Slug,
		"timezone": t.Timezone,
		"currency": t.Currency,
		"plan":     t.Plan,
		"settings": t.Settings,
	}
}

type createTenantRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *Handler) CreateTenant(c *gin.Context) {
	var req createTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := jwt.UserIDFromContext(c)
	tenant, err := h.useCases.RegisterTenant(c.Request.Context(), req.Name, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create tenant"})
		return
	}

	if err := h.onboarding.InitProgress(c.Request.Context(), tenant.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not init onboarding"})
		return
	}

	c.JSON(http.StatusCreated, tenantResponse(tenant))
}

type updateSettingsRequest struct {
	WelcomeMessage           string `json:"welcomeMessage"`
	BotName                  string `json:"botName"`
	CancellationMsg          string `json:"cancellationMessage"`
	ContactEmail             string `json:"contactEmail"`
	LateCancelHours          int    `json:"lateCancelHours"`
	AutoBlockAfterNoShows    int    `json:"autoBlockAfterNoShows"`
	AutoBlockAfterLateCancel int    `json:"autoBlockAfterLateCancel"`
	SendWarningBeforeBlock   bool   `json:"sendWarningBeforeBlock"`
}

type connectWhatsappRequest struct {
	WabaID             string `json:"wabaId"             binding:"required"`
	PhoneNumberID      string `json:"phoneNumberId"      binding:"required"`
	DisplayPhoneNumber string `json:"displayPhoneNumber" binding:"required"`
	AccessToken        string `json:"accessToken"        binding:"required"`
}

func (h *Handler) GetTenant(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)
	tenant, err := h.useCases.FindByID(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, tenantResponse(tenant))
}

func (h *Handler) GetTenantByUser(c *gin.Context) {
	userID := jwt.UserIDFromContext(c)
	tenant, err := h.useCases.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, tenantResponse(tenant))
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	if err := h.useCases.UpdateSettings(c.Request.Context(), tenantID, TenantSettings{
		WelcomeMessage:           req.WelcomeMessage,
		BotName:                  req.BotName,
		CancellationMsg:          req.CancellationMsg,
		ContactEmail:             req.ContactEmail,
		LateCancelHours:          req.LateCancelHours,
		AutoBlockAfterNoShows:    req.AutoBlockAfterNoShows,
		AutoBlockAfterLateCancel: req.AutoBlockAfterLateCancel,
		SendWarningBeforeBlock:   req.SendWarningBeforeBlock,
	}); err != nil {
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
		"phoneNumberId":      cfg.PhoneNumberID,
		"displayPhoneNumber": cfg.DisplayPhoneNumber,
		"isActive":           cfg.IsActive,
	})
}
