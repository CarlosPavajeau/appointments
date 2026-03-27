package onboarding_step_barber

import (
	"database/sql"
	"net/http"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	stepBarber   int32 = 2
	stepWhatsApp int32 = 4
)

type Request struct {
	Name        string `json:"name"        binding:"required,min=2"`
	WorkingDays []int  `json:"workingDays" binding:"required,min=1"`
	StartTime   string `json:"startTime"   binding:"required"`
	EndTime     string `json:"endTime"     binding:"required"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/onboarding/step/2" }

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse("15:04:05", s); err == nil {
		return t, nil
	}
	return time.Parse("15:04", s)
}

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	progress, err := db.Query.FindOnboardingProgressByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if progress.CurrentStep < stepBarber {
		c.JSON(http.StatusForbidden, gin.H{"error": "step not available yet"})
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

	resourceID := uuid.New()
	if err := db.Query.InsertResource(c.Request.Context(), h.DB.Primary(), db.InsertResourceParams{
		ID:        resourceID,
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      "barber",
		AvatarUrl: sql.NullString{},
		SortOrder: 1,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	for _, day := range req.WorkingDays {
		if err := db.Query.UpsertWorkingHours(c.Request.Context(), h.DB.Primary(), db.UpsertWorkingHoursParams{
			ID:         uuid.New(),
			ResourceID: resourceID,
			DayOfWeek:  int16(day),
			StartTime:  startTime,
			EndTime:    endTime,
			IsActive:   true,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}

	if err := db.Query.AdvanceOnboardingStep(c.Request.Context(), h.DB.Primary(), db.AdvanceOnboardingStepParams{
		TenantID:    tenantID,
		CurrentStep: stepWhatsApp,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"nextStep": stepBarber + 1})
}
