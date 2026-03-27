package resources_list_overrides

import (
	"net/http"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID        uuid.UUID `json:"id"`
	Date      string    `json:"date"`
	IsDayOff  bool      `json:"isDayOff"`
	StartTime *string   `json:"startTime"`
	EndTime   *string   `json:"endTime"`
	Reason    string    `json:"reason"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/resources/:id/overrides" }

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource id"})
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

	from := time.Now()
	to := from.AddDate(0, 0, 30)

	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}

	overrides, err := db.Query.FindResourceScheduleOverrides(c.Request.Context(), h.DB.Primary(), db.FindResourceScheduleOverridesParams{
		ResourceID: id,
		Date:       from,
		Date_2:     to,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch overrides"})
		return
	}

	response := make([]Response, len(overrides))
	for i, o := range overrides {
		response[i] = Response{
			ID:        o.ID,
			Date:      o.Date.Format("2006-01-02"),
			IsDayOff:  o.IsDayOff,
			StartTime: nullableString(o.StartTime),
			EndTime:   nullableString(o.EndTime),
			Reason:    o.Reason,
		}
	}

	c.JSON(http.StatusOK, response)
}
