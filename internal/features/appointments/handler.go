package appointments

import (
	"errors"
	"net/http"
	"time"
	apperrors "wappiz/internal/shared/errors"
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
	g := r.Group("/api/v1/appointments")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("", h.Search)
		g.PUT("/:id/status", h.UpdateStatus)
	}
}

type appointmentResponse struct {
	ID             uuid.UUID `json:"id"`
	ResourceName   string    `json:"resourceName"`
	ServiceName    string    `json:"serviceName"`
	CustomerName   string    `json:"customerName"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	Status         string    `json:"status"`
	PriceAtBooking float64   `json:"priceAtBooking"`
}

func (h *Handler) Search(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date query parameter is required (YYYY-MM-DD)"})
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be in YYYY-MM-DD format"})
		return
	}

	filters, ok := parseListFilters(c)
	if !ok {
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	appts, err := h.useCases.Search(c.Request.Context(), tenantID, date, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch appointments"})
		return
	}

	result := make([]appointmentResponse, len(appts))
	for i, a := range appts {
		result[i] = appointmentResponse{
			ID:             a.ID,
			ResourceName:   a.ResourceName,
			ServiceName:    a.ServiceName,
			CustomerName:   a.CustomerName,
			StartsAt:       a.StartsAt,
			EndsAt:         a.EndsAt,
			Status:         a.Status,
			PriceAtBooking: a.PriceAtBooking,
		}
	}
	c.JSON(http.StatusOK, result)
}

type updateStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid appointment ID"})
		return
	}

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status is required"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	updatedBy := jwt.UserIDFromContext(c)

	err = h.useCases.UpdateStatus(c.Request.Context(), id, tenantID, req.Status, updatedBy, req.Reason)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "appointment not found"})
			return
		}
		if errors.Is(err, ErrInvalidTransition) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid status transition"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseListFilters(c *gin.Context) (ListFilters, bool) {
	var filters ListFilters

	for _, raw := range c.QueryArray("resource") {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource ID: " + raw})
			return filters, false
		}
		filters.ResourceIDs = append(filters.ResourceIDs, id)
	}

	for _, raw := range c.QueryArray("service") {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service ID: " + raw})
			return filters, false
		}
		filters.ServiceIDs = append(filters.ServiceIDs, id)
	}

	if raw := c.Query("customer"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID: " + raw})
			return filters, false
		}
		filters.CustomerID = &id
	}

	return filters, true
}
