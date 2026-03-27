package resources_create_override

import (
	"database/sql"
	"net/http"
	"time"
	"wappiz/pkg/db"
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

func nullTime(s *string) sql.NullTime {
	if s == nil || *s == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse("15:04:05", *s)
	if err != nil {
		t, err = time.Parse("15:04", *s)
		if err != nil {
			return sql.NullTime{}
		}
	}
	return sql.NullTime{Time: t, Valid: true}
}

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

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date must be in YYYY-MM-DD format"})
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

	if err := db.Query.InsertScheduleOverride(c.Request.Context(), h.DB.Primary(), db.InsertScheduleOverrideParams{
		ID:         uuid.New(),
		ResourceID: id,
		Date:       date,
		IsDayOff:   req.IsDayOff,
		StartTime:  nullTime(req.StartTime),
		EndTime:    nullTime(req.EndTime),
		Reason:     sql.NullString{String: req.Reason, Valid: req.Reason != ""},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}
