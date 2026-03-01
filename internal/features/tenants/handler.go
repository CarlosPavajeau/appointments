package tenants

import (
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
	}

	protected := r.Group("/api/v1/tenants")

	protected.GET("/me", h.GetMe)
	protected.PUT("/settings", h.UpdateSettings)
	protected.POST("/whatsapp", h.ConnectWhatsapp)
}

type registerRequest struct {
	Name     string `json:"name"     binding:"required,min=2"`
	Timezone string `json:"timezone" binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
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

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	output, err := h.useCases.RegisterTenant(c.Request.Context(), RegisterTenantInput{
		Name:     req.Name,
		Timezone: req.Timezone,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "registration failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"tenant": gin.H{
			"id":   output.Tenant.ID,
			"name": output.Tenant.Name,
			"slug": output.Tenant.Slug,
			"plan": output.Tenant.Plan,
		},
	})
}

func (h *Handler) GetMe(c *gin.Context) {
	rawTenantID := c.Query("tenant_id")
	tenantID, err := uuid.Parse(rawTenantID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

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

	rawTenantID := c.Query("tenant_id")
	tenantID, parseErr := uuid.Parse(rawTenantID)

	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

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

	rawTenantID := c.Query("tenant_id")
	tenantID, err := uuid.Parse(rawTenantID)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}

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
