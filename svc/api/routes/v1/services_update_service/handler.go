package services_update_service

import (
	"database/sql"
	"fmt"
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	Name            string  `json:"name"            binding:"required,min=2"`
	Description     string  `json:"description"`
	DurationMinutes int32   `json:"durationMinutes" binding:"required,min=1"`
	BufferMinutes   int32   `json:"bufferMinutes"`
	Price           float64 `json:"price"           binding:"required,min=0"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string {
	return http.MethodPut
}

func (h *Handler) Path() string {
	return "/v1/services/:id"
}

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service id"})
		return
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := db.Query.UpdateService(c.Request.Context(), h.DB.Primary(), db.UpdateServiceParams{
		Name:            req.Name,
		Description:     sql.NullString{String: req.Description},
		DurationMinutes: req.DurationMinutes,
		BufferMinutes:   req.BufferMinutes,
		Price:           fmt.Sprint(req.Price),
		SortOrder:       1,
		ID:              id,
		TenantID:        tenantID,
	}); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
