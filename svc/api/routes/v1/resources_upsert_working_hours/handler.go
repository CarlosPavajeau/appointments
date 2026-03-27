package resources_upsert_working_hours

import (
	"net/http"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	DayOfWeek int    `json:"dayOfWeek" binding:"min=0,max=6"`
	StartTime string `json:"startTime" binding:"required"`
	EndTime   string `json:"endTime"   binding:"required"`
	IsActive  bool   `json:"isActive"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPut }
func (h *Handler) Path() string   { return "/v1/resources/:id/working-hours" }

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse("15:04:05", s); err == nil {
		return t, nil
	}
	return time.Parse("15:04", s)
}

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid startTime format, use HH:MM or HH:MM:SS"})
		return
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid endTime format, use HH:MM or HH:MM:SS"})
		return
	}

	if err := db.Query.UpsertWorkingHours(c.Request.Context(), h.DB.Primary(), db.UpsertWorkingHoursParams{
		ID:         uuid.New(),
		ResourceID: id,
		DayOfWeek:  int16(req.DayOfWeek),
		StartTime:  startTime,
		EndTime:    endTime,
		IsActive:   req.IsActive,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
