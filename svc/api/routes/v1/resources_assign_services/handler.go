package resources_assign_services

import (
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	ServiceIDs []uuid.UUID `json:"serviceIds" binding:"required"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPut }
func (h *Handler) Path() string   { return "/v1/resources/:id/services" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
		return
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if err := db.Query.DeleteResourceService(c.Request.Context(), h.DB.Primary(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign services"})
		return
	}

	for _, serviceID := range req.ServiceIDs {
		if err := db.Query.InsertResourceService(c.Request.Context(), h.DB.Primary(), db.InsertResourceServiceParams{
			ResourceID: id,
			ServiceID:  serviceID,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to assign services"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "services assigned"})
}
