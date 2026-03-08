// internal/features/onboarding/handler.go

package onboarding

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"appointments/internal/shared/jwt"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

func (h *Handler) RegisterRoutes(r gin.IRouter) {
	g := r.Group("/api/v1/onboarding")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("/progress", h.GetProgress)
		g.GET("/templates", h.GetTemplates)
		g.POST("/step/2", h.CompleteStepBarber)
		g.POST("/step/3", h.CompleteStepServices)
		g.POST("/step/4", h.CompleteStepWhatsApp)
	}

	// Rutas de administración interna — solo superadmin
	admin := r.Group("/api/v1/admin")
	admin.Use(jwt.AuthMiddleware(), SuperAdminMiddleware())
	{
		admin.GET("/activations", h.ListActivations)
		admin.POST("/activations/:id/activate", h.ActivateTenant)
	}
}

// ── Request types ─────────────────────────────────────────────────

type stepBarberRequest struct {
	Name        string `json:"name"         binding:"required,min=2"`
	WorkingDays []int  `json:"working_days" binding:"required,min=1"`
	StartTime   string `json:"start_time"   binding:"required"`
	EndTime     string `json:"end_time"     binding:"required"`
}

type stepServiceItemRequest struct {
	Name            string  `json:"name"             binding:"required,min=2"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,min=1"`
	BufferMinutes   int     `json:"buffer_minutes"`
	Price           float64 `json:"price"            binding:"required,min=0"`
}

type stepServicesRequest struct {
	Services []stepServiceItemRequest `json:"services" binding:"required,min=1"`
}

type stepWhatsAppRequest struct {
	ContactEmail string `json:"contact_email" binding:"required,email"`
	Notes        string `json:"notes"`
}

type activateRequest struct {
	PhoneNumberID      string `json:"phone_number_id"      binding:"required"`
	DisplayPhoneNumber string `json:"display_phone_number" binding:"required"`
	WABAID             string `json:"waba_id"              binding:"required"`
	AccessToken        string `json:"access_token"         binding:"required"`
}

// ── Handlers ──────────────────────────────────────────────────────

func (h *Handler) GetProgress(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	progress, err := h.useCases.GetProgress(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"current_step": progress.CurrentStep,
		"is_completed": progress.IsCompleted(),
	})
}

func (h *Handler) GetTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"templates": h.useCases.GetTemplates(),
	})
}

func (h *Handler) CompleteStepBarber(c *gin.Context) {
	var req stepBarberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.CompleteStepBarber(c.Request.Context(), StepBarberInput{
		TenantID:    tenantID,
		Name:        req.Name,
		WorkingDays: req.WorkingDays,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
	}); err != nil {
		statusCode, msg := resolveError(err)
		c.JSON(statusCode, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"next_step": StepServices})
}

func (h *Handler) CompleteStepServices(c *gin.Context) {
	var req stepServicesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	items := make([]StepServiceItem, len(req.Services))
	for i, s := range req.Services {
		items[i] = StepServiceItem{
			Name:            s.Name,
			DurationMinutes: s.DurationMinutes,
			BufferMinutes:   s.BufferMinutes,
			Price:           s.Price,
		}
	}

	if err := h.useCases.CompleteStepServices(c.Request.Context(), StepServicesInput{
		TenantID: tenantID,
		Services: items,
	}); err != nil {
		statusCode, msg := resolveError(err)
		c.JSON(statusCode, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"next_step": StepWhatsApp})
}

func (h *Handler) CompleteStepWhatsApp(c *gin.Context) {
	var req stepWhatsAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.CompleteStepWhatsApp(c.Request.Context(), StepWhatsAppInput{
		TenantID:     tenantID,
		ContactEmail: req.ContactEmail,
		Notes:        req.Notes,
	}); err != nil {
		statusCode, msg := resolveError(err)
		if statusCode == http.StatusInternalServerError {
			log.Printf("onboarding: CompleteStepWhatsApp 500 tenant_id=%s err=%v", tenantID, err)
		}
		c.JSON(statusCode, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"redirect": "/dashboard"})
}

// ── Admin handlers ────────────────────────────────────────────────

func (h *Handler) ListActivations(c *gin.Context) {
	activations, err := h.useCases.ListActivations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch activations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"activations": activations})
}

func (h *Handler) ActivateTenant(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}

	var req activateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.useCases.ActivateTenant(c.Request.Context(), ActivateTenantInput{
		TenantID:           tenantID,
		PhoneNumberID:      req.PhoneNumberID,
		DisplayPhoneNumber: req.DisplayPhoneNumber,
		WABAID:             req.WABAID,
		AccessToken:        req.AccessToken,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "tenant activated"})
}

// ── Middleware superadmin ─────────────────────────────────────────

func SuperAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role != "superadmin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// ── Error resolver ────────────────────────────────────────────────

func resolveError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrStepNotAvailable):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, ErrServicesRequired),
		errors.Is(err, ErrBarberRequired):
		return http.StatusBadRequest, err.Error()
	default:
		return http.StatusInternalServerError, "internal server error"
	}
}
