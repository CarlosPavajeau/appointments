package resources_create_override

import (
	"database/sql"
	"net/http"
	"time"
	"wappiz/pkg/codes"
	"wappiz/pkg/db"
	"wappiz/pkg/fault"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Request struct {
	Date      string  `json:"date"      binding:"required"`
	IsDayOff  bool    `json:"isDayOff"`
	StartTime *string `json:"startTime"`
	EndTime   *string `json:"endTime"`
	Reason    string  `json:"reason"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/resources/:id/overrides" }

func nullTime(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	t, err := time.Parse("15:04:05", *s)
	if err != nil {
		t, err = time.Parse("15:04", *s)
		if err != nil {
			return sql.NullString{}
		}
	}
	return sql.NullString{String: t.Format("15:04:05"), Valid: true}
}

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

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid request body"),
			fault.Public("Los datos enviados son inválidos"),
		))
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.Error(fault.Wrap(err,
			fault.Code(codes.ErrorsBadRequest),
			fault.Internal("invalid date format"),
			fault.Public("El campo 'date' debe tener formato YYYY-MM-DD"),
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

	if err := db.Query.InsertScheduleOverride(c.Request.Context(), h.DB.Primary(), db.InsertScheduleOverrideParams{
		ID:         uuid.New(),
		ResourceID: id,
		Date:       date,
		IsDayOff:   req.IsDayOff,
		StartTime:  nullTime(req.StartTime),
		EndTime:    nullTime(req.EndTime),
		Reason:     sql.NullString{String: req.Reason, Valid: req.Reason != ""},
	}); err != nil {
		c.Error(fault.Wrap(err, fault.Internal("failed to create schedule override")))
		return
	}

	c.Status(http.StatusCreated)
}
