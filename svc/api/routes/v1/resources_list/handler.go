package resources_list

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
func (h *Handler) Path() string   { return "/v1/resources" }

func (h *Handler) Handle(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)

	resources, err := db.Query.FindResourcesByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch resources"})
		return
	}

	response := make([]Response, len(resources))
	for i, r := range resources {
		whs, err := db.Query.FindResourceWorkingHours(c.Request.Context(), h.DB.Primary(), r.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch working hours"})
			return
		}

		whResponse := make([]WorkingHoursResponse, len(whs))
		for j, wh := range whs {
			whResponse[j] = WorkingHoursResponse{
				ID:        wh.ID,
				DayOfWeek: wh.DayOfWeek,
				DayName:   dayNames[wh.DayOfWeek],
				StartTime: wh.StartTime,
				EndTime:   wh.EndTime,
				IsActive:  wh.IsActive,
			}
		}

		response[i] = Response{
			ID:           r.ID,
			Name:         r.Name,
			Type:         r.Type,
			AvatarURL:    r.AvatarUrl,
			SortOrder:    r.SortOrder,
			WorkingHours: whResponse,
		}
	}

	c.JSON(http.StatusOK, response)
}
