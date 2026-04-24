package resources_delete_working_hours

import (
	"net/http"
	"wappiz/pkg/codes"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
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
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid resource id"),
			fault.Public("Id del recurso inválido"),
		))
		return
	}
	whID, err := uuid.Parse(c.Param("whid"))
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid working hours id"),
			fault.Public("Id del horario inválido"),
		))
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	r, err := db.Query.FindResourceById(c.Request.Context(), h.DB.Primary(), id)
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsNotFound),
			fault.Internal("resource not found"),
			fault.Public("El recurso no existe"),
		))
		return
	}
	if r.TenantID != tenantID {
		c.Error(fault.New("resource not found",
			fault.Code(codes.ErrorsNotFound),
			fault.Internal("resource belongs to a different tenant"),
			fault.Public("El recurso no existe"),
		))
		return
	}

	if err := db.Query.DeleteWorkingHour(c.Request.Context(), h.DB.Primary(), db.DeleteWorkingHourParams{
		ID:         whID,
		ResourceID: id,
	}); err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to delete working hours")))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "working hours deleted"})
}
