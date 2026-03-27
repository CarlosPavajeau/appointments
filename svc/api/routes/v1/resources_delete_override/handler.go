package resources_delete_override

import (
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodDelete }
func (h *Handler) Path() string   { return "/v1/resources/:id/overrides/:oid" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}
	oid, err := uuid.Parse(c.Param("oid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid override id"})
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

	if err := db.Query.DeleteScheduleOverride(c.Request.Context(), h.DB.Primary(), db.DeleteScheduleOverrideParams{
		ID:         oid,
		ResourceID: id,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete override"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "override deleted"})
}
