package admin

import (
	"net/http"
	"time"

	"wappiz/internal/shared/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

// RegisterRoutes mounts all admin endpoints under /api/v1/admin.
// All routes require a valid JWT and the superadmin role.
//
//	GET  /api/v1/admin/activations              — list pending WhatsApp activations
//	POST /api/v1/admin/activations/:id/activate — activate a tenant's WhatsApp
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/admin")
	g.Use(jwt.AuthMiddleware(), superAdminMiddleware())
	{
		g.GET("/activations", h.ListActivations)
		g.POST("/activations/:id/activate", h.ActivateTenant)
	}
}

type activationResponse struct {
	TenantID     uuid.UUID `json:"tenantId"`
	TenantName   string    `json:"tenantName"`
	ContactEmail string    `json:"contactEmail"`
	Notes        string    `json:"notes"`
	Status       string    `json:"status"`
	RequestedAt  string    `json:"requestedAt"`
}

type activateRequest struct {
	PhoneNumberID      string `json:"phoneNumberId"      binding:"required"`
	DisplayPhoneNumber string `json:"displayPhoneNumber" binding:"required"`
	WABAID             string `json:"wabaId"             binding:"required"`
	AccessToken        string `json:"accessToken"        binding:"required"`
}

// ListActivations returns all tenants pending WhatsApp activation.
// GET /api/v1/admin/activations
func (h *Handler) ListActivations(c *gin.Context) {
	activations, err := h.useCases.ListActivations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch activations"})
		return
	}

	result := make([]activationResponse, len(activations))
	for i, a := range activations {
		result[i] = activationResponse{
			TenantID:     a.TenantID,
			TenantName:   a.TenantName,
			ContactEmail: a.ContactEmail,
			Notes:        a.Notes,
			Status:       string(a.Status),
			RequestedAt:  a.RequestedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, result)
}

// ActivateTenant activates the WhatsApp config for a given tenant.
// POST /api/v1/admin/activations/:id/activate
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

// superAdminMiddleware restricts access to users with the superadmin role.
func superAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
