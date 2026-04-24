package tenants_update

import (
	"encoding/json"
	"net/http"
	"wappiz/pkg/codes"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
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

func (h *Handler) Method() string { return http.MethodPut }
func (h *Handler) Path() string   { return "/v1/tenants/settings" }

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

	tenant, err := db.Query.FindTenantByID(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsNotFound),
			fault.Internal("tenant not found"),
			fault.Public("La cuenta no fue encontrada"),
		))
		return
	}

	newSettings, err := json.Marshal(req)
	if err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to serialize settings")))
		return
	}

	if err := db.Query.UpdateTenant(c.Request.Context(), h.DB.Primary(), db.UpdateTenantParams{
		Name:     tenant.Name,
		Timezone: tenant.Timezone,
		Settings: newSettings,
		ID:       tenantID,
	}); err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to update tenant settings")))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}
