package onboarding_step_resource

import (
	"context"
	"database/sql"
	"net/http"
	"time"
	"wappiz/pkg/codes"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	stepResource int32 = 2
	stepWhatsApp int32 = 4
)

type Request struct {
	Name        string `json:"name"        binding:"required,min=2"`
	Type        string `json:"type"      binding:"required"`
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
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid request body"),
			fault.Public("Los datos enviados son inválidos"),
		))
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	progress, err := db.Query.FindOnboardingProgressByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to fetch onboarding progress")))
		return
	}
	if progress.CurrentStep < stepResource {
		c.Error(fault.New("onboarding step not available",
			fault.Code(codes.ErrorsForbidden),
			fault.Internal("step not available yet"),
			fault.Public("Este paso aún no está disponible"),
		))
		return
	}

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid startTime format"),
			fault.Public("El campo 'startTime' debe tener formato HH:MM o HH:MM:SS"),
		))
		return
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid endTime format"),
			fault.Public("El campo 'endTime' debe tener formato HH:MM o HH:MM:SS"),
		))
		return
	}

	err = db.Tx(c.Request.Context(), h.DB.Primary(), func(ctx context.Context, txx db.DBTX) error {
		resourceID := uuid.New()
		if err := db.Query.InsertResource(c.Request.Context(), txx, db.InsertResourceParams{
			ID:        resourceID,
			TenantID:  tenantID,
			Name:      req.Name,
			Type:      req.Type,
			AvatarUrl: sql.NullString{},
			SortOrder: 1,
		}); err != nil {
			return fault.Wrap(err, fault.Internal("failed to create resource"))
		}

		for _, day := range req.WorkingDays {
			if err := db.Query.UpsertWorkingHours(c.Request.Context(), txx, db.UpsertWorkingHoursParams{
				ID:         uuid.New(),
				ResourceID: resourceID,
				DayOfWeek:  int16(day),
				StartTime:  startTime,
				EndTime:    endTime,
				IsActive:   true,
			}); err != nil {
				return fault.Wrap(err, fault.Internal("failed to save working hours"))
			}
		}

		if err := db.Query.AdvanceOnboardingStep(c.Request.Context(), txx, db.AdvanceOnboardingStepParams{
			TenantID:    tenantID,
			CurrentStep: stepWhatsApp,
		}); err != nil {
			return fault.Wrap(err, fault.Internal("failed to advance onboarding step"))
		}

		return nil
	})

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"nextStep": stepResource + 1})
}
