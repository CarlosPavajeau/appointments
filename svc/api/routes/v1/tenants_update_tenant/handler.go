package tenants_update_tenant

import (
	"encoding/json"
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type Request struct {
	WelcomeMessage           string `json:"welcomeMessage"`
	BotName                  string `json:"botName"`
	CancellationMsg          string `json:"cancellationMessage"`
	ContactEmail             string `json:"contactEmail"`
	LateCancelHours          int    `json:"lateCancelHours"`
	AutoBlockAfterNoShows    int    `json:"autoBlockAfterNoShows"`
	AutoBlockAfterLateCancel int    `json:"autoBlockAfterLateCancel"`
	SendWarningBeforeBlock   bool   `json:"sendWarningBeforeBlock"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string {
	return http.MethodPut
}

func (h *Handler) Path() string {
	return "/v1/tenants"
}

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)
	tenant, err := db.Query.FindTenantByID(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	newSettings, err := json.Marshal(req)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.Query.UpdateTenant(c.Request.Context(), h.DB.Primary(), db.UpdateTenantParams{
		Name:     tenant.Name,
		Timezone: tenant.Timezone,
		Settings: newSettings,
		ID:       tenantID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}
