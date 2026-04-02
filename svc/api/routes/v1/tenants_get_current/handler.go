package tenants_get_current

import (
	"encoding/json"
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID       uuid.UUID         `json:"id"`
	Name     string            `json:"name"`
	Slug     string            `json:"slug"`
	TimeZone string            `json:"time_zone"`
	Currency string            `json:"currency"`
	Plan     string            `json:"plan"`
	Settings db.TenantSettings `json:"settings"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string {
	return http.MethodGet
}

func (h *Handler) Path() string {
	return "/v1/tenants/me"
}

func (h *Handler) Handle(c *gin.Context) {
	tenantID := jwt.TenantIDFromContext(c)
	tenant, err := db.Query.FindTenantByID(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	var settings db.TenantSettings
	if err := json.Unmarshal(tenant.Settings, &settings); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{
		ID:       tenant.ID,
		Name:     tenant.Name,
		Slug:     tenant.Slug,
		TimeZone: tenant.Timezone,
		Currency: tenant.Currency,
		Plan:     tenant.Plan,
		Settings: settings,
	})
}
