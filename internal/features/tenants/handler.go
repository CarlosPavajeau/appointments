package tenants

import (
	"net/http"

	"wappiz/internal/shared/jwt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

// RegisterRoutes mounts protected tenant management endpoints.
//
//	GET  /api/v1/tenants/me        — get current tenant info
//	PUT  /api/v1/tenants/settings  — update tenant settings
//	POST /api/v1/tenants/whatsapp  — connect WhatsApp config
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	protected := r.Group("/api/v1/tenants")
	protected.Use(jwt.AuthMiddleware())
	{
		protected.GET("/me", h.GetTenant)
		protected.PUT("/settings", h.UpdateSettings)
		protected.POST("/whatsapp", h.ConnectWhatsapp)
	}
}

type updateSettingsRequest struct {
	WelcomeMessage  string `json:"welcomeMessage"`
	BotName         string `json:"botName"`
	CancellationMsg string `json:"cancellationMessage"`
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

	c.JSON(http.StatusOK, gin.H{
		"id":       tenant.ID,
		"name":     tenant.Name,
		"slug":     tenant.Slug,
		"timezone": tenant.Timezone,
		"currency": tenant.Currency,
		"plan":     tenant.Plan,
		"settings": tenant.Settings,
	})
}

func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	if err := h.useCases.UpdateSettings(c.Request.Context(), tenantID, TenantSettings{
		WelcomeMessage:  req.WelcomeMessage,
		BotName:         req.BotName,
		CancellationMsg: req.CancellationMsg,
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
