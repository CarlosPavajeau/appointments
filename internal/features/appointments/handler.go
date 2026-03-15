package appointments

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

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	g := r.Group("/api/v1/appointments")
	g.Use(jwt.AuthMiddleware())
	{
		g.GET("", h.ListByDate)
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

func (h *Handler) ListByDate(c *gin.Context) {
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

	tenantID := jwt.TenantIDFromContext(c)

	appts, err := h.useCases.GetByDateWithDetails(c.Request.Context(), tenantID, date)
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
