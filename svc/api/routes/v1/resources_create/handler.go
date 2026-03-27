package resources_create

import (
	"database/sql"
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	Name      string `json:"name"      binding:"required,min=2"`
	Type      string `json:"type"      binding:"required"`
	AvatarURL string `json:"avatarUrl"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/resources" }

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := db.Query.InsertResource(c.Request.Context(), h.DB.Primary(), db.InsertResourceParams{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Name:      req.Name,
		Type:      req.Type,
		AvatarUrl: sql.NullString{String: req.AvatarURL, Valid: req.AvatarURL != ""},
		SortOrder: 1,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}
