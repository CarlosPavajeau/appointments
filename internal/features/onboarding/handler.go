package onboarding

import (
	"errors"
	"log"
	"net/http"

	"appointments/internal/shared/jwt"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

// RegisterRoutes mounts all onboarding endpoints under /api/v1/onboarding.
// All routes require a valid JWT.
//
//	GET  /api/v1/onboarding/progress — get current onboarding step
//	GET  /api/v1/onboarding/templates — list service templates
//	POST /api/v1/onboarding/step/2   — complete barber step
//	POST /api/v1/onboarding/step/3   — complete services step
//	POST /api/v1/onboarding/step/4   — complete WhatsApp step
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
}

type stepBarberRequest struct {
	Name        string `json:"name"        binding:"required,min=2"`
	WorkingDays []int  `json:"workingDays" binding:"required,min=1"`
	StartTime   string `json:"startTime"   binding:"required"`
	EndTime     string `json:"endTime"     binding:"required"`
}

type stepServiceItemRequest struct {
	Name            string  `json:"name"            binding:"required,min=2"`
	DurationMinutes int     `json:"durationMinutes" binding:"required,min=1"`
	BufferMinutes   int     `json:"bufferMinutes"`
	Price           float64 `json:"price"           binding:"required,min=0"`
}

type stepServicesRequest struct {
	Services []stepServiceItemRequest `json:"services" binding:"required,min=1"`
}

type stepWhatsAppRequest struct {
	ContactEmail string `json:"contactEmail" binding:"required,email"`
	Notes        string `json:"notes"`
}

func (h *Handler) GetProgress(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	progress, err := h.useCases.GetProgress(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"currentStep": progress.CurrentStep,
		"isCompleted": progress.IsCompleted(),
	})
}

func (h *Handler) GetTemplates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"templates": h.useCases.GetTemplates()})
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

	c.JSON(http.StatusOK, gin.H{"nextStep": StepServices})
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

	c.JSON(http.StatusOK, gin.H{"nextStep": StepWhatsApp})
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
