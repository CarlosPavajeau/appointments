package onboarding_step_services

import (
	"fmt"
	"net/http"
	"wappiz/pkg/codes"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	stepServices int32 = 3
	stepWhatsApp int32 = 4
)

type ServiceItem struct {
	Name            string  `json:"name"            binding:"required,min=2"`
	DurationMinutes int32   `json:"durationMinutes" binding:"required,min=1"`
	BufferMinutes   int32   `json:"bufferMinutes"`
	Price           float64 `json:"price"           binding:"required,min=0"`
}

type Request struct {
	Services []ServiceItem `json:"services" binding:"required,min=1"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/onboarding/step/3" }

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
	if progress.CurrentStep < stepServices {
		c.Error(fault.New("onboarding step not available",
			fault.Code(codes.ErrorsForbidden),
			fault.Internal("step not available yet"),
			fault.Public("Este paso aún no está disponible"),
		))
		return
	}

	resources, err := db.Query.FindResourcesByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to fetch resources")))
		return
	}
	if len(resources) == 0 {
		c.Error(fault.New("no resources found",
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("no resources found for tenant"),
			fault.Public("Primero debes completar el paso del barbero"),
		))
		return
	}

	firstResourceID := resources[0].ID

	for i, item := range req.Services {
		serviceID := uuid.New()
		if err := db.Query.InsertService(c.Request.Context(), h.DB.Primary(), db.InsertServiceParams{
			ID:              serviceID,
			TenantID:        tenantID,
			Name:            item.Name,
			DurationMinutes: item.DurationMinutes,
			BufferMinutes:   item.BufferMinutes,
			Price:           fmt.Sprintf("%g", item.Price),
			SortOrder:       int32(i + 1),
		}); err != nil {
			c.Error(fault.Wrap(err, fault.Internal("failed to create service")))
			return
		}

		if err := db.Query.InsertResourceService(c.Request.Context(), h.DB.Primary(), db.InsertResourceServiceParams{
			ResourceID: firstResourceID,
			ServiceID:  serviceID,
		}); err != nil {
			c.Error(fault.Wrap(err, fault.Internal("failed to assign service to resource")))
			return
		}
	}

	if err := db.Query.AdvanceOnboardingStep(c.Request.Context(), h.DB.Primary(), db.AdvanceOnboardingStepParams{
		TenantID:    tenantID,
		CurrentStep: stepWhatsApp,
	}); err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to advance onboarding step")))
		return
	}

	c.JSON(http.StatusOK, gin.H{"nextStep": stepServices + 1})
}
