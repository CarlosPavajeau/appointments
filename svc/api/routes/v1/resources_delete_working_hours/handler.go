package resources_delete_working_hours

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
func (h *Handler) Path() string   { return "/v1/resources/:id/working-hours/:whid" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}
	whID, err := uuid.Parse(c.Param("whid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid working hours id"})
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

	if err := db.Query.DeleteWorkingHour(c.Request.Context(), h.DB.Primary(), db.DeleteWorkingHourParams{
		ID:         whID,
		ResourceID: id,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete working hours"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "working hours deleted"})
}
