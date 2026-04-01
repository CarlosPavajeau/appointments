package onboarding_get_progress

import (
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"
	"wappiz/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/onboarding/progress" }

const stepAccount = 1

func (h *Handler) Handle(c *gin.Context) {
	tenantID, ok := jwt.TenantIDFromContextOK(c)
	if !ok {
		c.JSON(http.StatusOK, gin.H{
			"currentStep": stepAccount,
			"isCompleted": false,
		})
		return
	}

	progress, err := db.Query.FindOnboardingProgressByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		logger.Error("failed to find onboarding progress by tenant",
			"tenant_id", tenantID,
			"err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch progress"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"currentStep": progress.CurrentStep,
		"isCompleted": progress.CompletedAt.Valid,
	})
}
