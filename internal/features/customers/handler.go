package customers

import (
	"net/http"
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

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/customers")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("", h.List)
		g.GET("/:id", h.GetByID)
		g.POST("/:id/block", h.Block)
		g.POST("/:id/unblock", h.Unblock)
	}
}

type customerResponse struct {
	ID          uuid.UUID `json:"id"`
	PhoneNumber string    `json:"phoneNumber"`
	Name        *string   `json:"name"`
	DisplayName string    `json:"displayName"`
	IsBlocked   bool      `json:"isBlocked"`
}

func toResponse(c Customer) customerResponse {
	return customerResponse{
		ID:          c.ID,
		PhoneNumber: c.PhoneNumber,
		Name:        c.Name,
		DisplayName: c.DisplayName(),
		IsBlocked:   c.IsBlocked,
	}
}

func (h *Handler) List(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	customers, err := h.useCases.GetAll(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch customers"})
		return
	}

	result := make([]customerResponse, len(customers))
	for i, cu := range customers {
		result[i] = toResponse(cu)
	}
	c.JSON(http.StatusOK, gin.H{"customers": result})
}

func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	customer, err := h.useCases.GetByID(c.Request.Context(), id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}

	c.JSON(http.StatusOK, toResponse(*customer))
}

func (h *Handler) Block(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.Block(c.Request.Context(), id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to block customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customer blocked"})
}

func (h *Handler) Unblock(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.Unblock(c.Request.Context(), id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to unblock customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customer unblocked"})
}
