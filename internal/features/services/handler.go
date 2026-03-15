package services

import (
	"errors"
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
	g := r.Group("/api/v1/services")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
		g.PUT("/sort-order", h.UpdateSortOrder)
	}
}

type createRequest struct {
	Name            string  `json:"name"            binding:"required,min=2"`
	Description     string  `json:"description"`
	DurationMinutes int     `json:"durationMinutes" binding:"required,min=1"`
	BufferMinutes   int     `json:"bufferMinutes"`
	Price           float64 `json:"price"           binding:"required,min=0"`
}

type updateRequest struct {
	Name            string  `json:"name"            binding:"required,min=2"`
	Description     string  `json:"description"`
	DurationMinutes int     `json:"durationMinutes" binding:"required,min=1"`
	BufferMinutes   int     `json:"bufferMinutes"`
	Price           float64 `json:"price"           binding:"required,min=0"`
	SortOrder       int     `json:"sortOrder"`
}

type sortOrderRequest struct {
	Order []struct {
		ID        uuid.UUID `json:"id"        binding:"required"`
		SortOrder int       `json:"sortOrder" binding:"required"`
	} `json:"order" binding:"required,min=1"`
}

type serviceResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DurationMinutes int       `json:"durationMinutes"`
	BufferMinutes   int       `json:"bufferMinutes"`
	TotalMinutes    int       `json:"totalMinutes"`
	Price           float64   `json:"price"`
	SortOrder       int       `json:"sortOrder"`
}

func toResponse(s Service) serviceResponse {
	return serviceResponse{
		ID:              s.ID,
		Name:            s.Name,
		Description:     s.Description,
		DurationMinutes: s.DurationMinutes,
		BufferMinutes:   s.BufferMinutes,
		TotalMinutes:    s.TotalDuration(),
		Price:           s.Price,
		SortOrder:       s.SortOrder,
	}
}

func (h *Handler) List(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	svcs, err := h.useCases.GetAll(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch services"})
		return
	}

	result := make([]serviceResponse, len(svcs))
	for i, s := range svcs {
		result[i] = toResponse(s)
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	svc, err := h.useCases.Create(c.Request.Context(), CreateServiceInput{
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		BufferMinutes:   req.BufferMinutes,
		Price:           req.Price,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, toResponse(*svc))
}

func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service id"})
		return
	}

	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	svc, err := h.useCases.Update(c.Request.Context(), UpdateServiceInput{
		ID:              id,
		TenantID:        tenantID,
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		BufferMinutes:   req.BufferMinutes,
		Price:           req.Price,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		if isValidationError(err) {
			statusCode = http.StatusBadRequest
		}
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, toResponse(*svc))
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := h.useCases.Delete(c.Request.Context(), id, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete service"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "service deleted"})
}

func (h *Handler) UpdateSortOrder(c *gin.Context) {
	var req sortOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	items := make([]SortItem, len(req.Order))
	for i, o := range req.Order {
		items[i] = SortItem{ID: o.ID, SortOrder: o.SortOrder}
	}

	if err := h.useCases.UpdateSortOrder(c.Request.Context(), tenantID, items); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sort order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sort order updated"})
}

func isValidationError(err error) bool {
	var serviceError serviceError
	ok := errors.As(err, &serviceError)
	return ok
}
