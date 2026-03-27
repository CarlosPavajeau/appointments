package resources_get_services

import (
	"net/http"
	"strconv"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DurationMinutes int32     `json:"durationMinutes"`
	BufferMinutes   int32     `json:"bufferMinutes"`
	Price           float64   `json:"price"`
	SortOrder       int32     `json:"sortOrder"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/resources/:id/services" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	r, err := db.Query.FindResourceById(c.Request.Context(), h.DB.Primary(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}
	if r.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}

	services, err := db.Query.FindServicesByResourceID(c.Request.Context(), h.DB.Primary(), db.FindServicesByResourceIDParams{
		TenantID:   tenantID,
		ResourceID: id,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch services"})
		return
	}

	response := make([]Response, len(services))
	for i, s := range services {
		price, _ := strconv.ParseFloat(s.Price, 64)
		response[i] = Response{
			ID:              s.ID,
			Name:            s.Name,
			Description:     s.Description.String,
			DurationMinutes: s.DurationMinutes,
			BufferMinutes:   s.BufferMinutes,
			Price:           price,
			SortOrder:       s.SortOrder,
		}
	}

	c.JSON(http.StatusOK, response)
}
