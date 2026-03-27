package resources_get

import (
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var dayNames = [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

type WorkingHoursResponse struct {
	ID        uuid.UUID `json:"id"`
	DayOfWeek int16     `json:"dayOfWeek"`
	DayName   string    `json:"dayName"`
	StartTime string    `json:"startTime"`
	EndTime   string    `json:"endTime"`
	IsActive  bool      `json:"isActive"`
}

type Response struct {
	ID           uuid.UUID              `json:"id"`
	Name         string                 `json:"name"`
	Type         string                 `json:"type"`
	AvatarURL    string                 `json:"avatarUrl"`
	SortOrder    int32                  `json:"sortOrder"`
	WorkingHours []WorkingHoursResponse `json:"workingHours"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/resources/:id" }

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

	whs, err := db.Query.FindResourceWorkingHours(c.Request.Context(), h.DB.Primary(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch working hours"})
		return
	}

	whResponse := make([]WorkingHoursResponse, len(whs))
	for i, wh := range whs {
		whResponse[i] = WorkingHoursResponse{
			ID:        wh.ID,
			DayOfWeek: wh.DayOfWeek,
			DayName:   dayNames[wh.DayOfWeek],
			StartTime: wh.StartTime,
			EndTime:   wh.EndTime,
			IsActive:  wh.IsActive,
		}
	}

	c.JSON(http.StatusOK, Response{
		ID:           r.ID,
		Name:         r.Name,
		Type:         r.Type,
		AvatarURL:    r.AvatarUrl,
		SortOrder:    r.SortOrder,
		WorkingHours: whResponse,
	})
}
