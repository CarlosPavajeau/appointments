package appointments_search

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID             uuid.UUID `json:"id"`
	ResourceName   string    `json:"resourceName"`
	ServiceName    string    `json:"serviceName"`
	CustomerName   string    `json:"customerName"`
	StartsAt       time.Time `json:"startsAt"`
	EndsAt         time.Time `json:"endsAt"`
	Status         string    `json:"status"`
	PriceAtBooking float64   `json:"priceAtBooking"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/appointments" }

func (h *Handler) Handle(c *gin.Context) {
	fromStr := c.Query("from")
	toStr := c.Query("to")
	if fromStr == "" || toStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to query parameters are required (YYYY-MM-DD)"})
		return
	}
	fromDate, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be in YYYY-MM-DD format"})
		return
	}
	toDate, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must be in YYYY-MM-DD format"})
		return
	}
	if toDate.Before(fromDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "to must not be before from"})
		return
	}

	var resourceIDs, serviceIDs []uuid.UUID
	var customerID *uuid.UUID
	var statuses []string

	for _, raw := range c.QueryArray("resource") {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource ID: " + raw})
			return
		}
		resourceIDs = append(resourceIDs, id)
	}
	for _, raw := range c.QueryArray("service") {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid service ID: " + raw})
			return
		}
		serviceIDs = append(serviceIDs, id)
	}
	if raw := c.Query("customer"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer ID: " + raw})
			return
		}
		customerID = &id
	}
	statuses = c.QueryArray("status")

	tenantID := jwt.TenantIDFromContext(c)

	dayStart := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, fromDate.Location())
	dayEnd := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, toDate.Location()).Add(24 * time.Hour)

	args := []interface{}{tenantID, dayStart, dayEnd}
	idx := 4

	var extraClauses []string

	if len(resourceIDs) > 0 {
		placeholders := make([]string, len(resourceIDs))
		for i, id := range resourceIDs {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, id)
			idx++
		}
		extraClauses = append(extraClauses, fmt.Sprintf("a.resource_id IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(serviceIDs) > 0 {
		placeholders := make([]string, len(serviceIDs))
		for i, id := range serviceIDs {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, id)
			idx++
		}
		extraClauses = append(extraClauses, fmt.Sprintf("a.service_id IN (%s)", strings.Join(placeholders, ",")))
	}
	if customerID != nil {
		extraClauses = append(extraClauses, fmt.Sprintf("a.customer_id = $%d", idx))
		args = append(args, *customerID)
		idx++
	}
	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, s := range statuses {
			placeholders[i] = fmt.Sprintf("$%d", idx)
			args = append(args, s)
			idx++
		}
		extraClauses = append(extraClauses, fmt.Sprintf("a.status IN (%s)", strings.Join(placeholders, ",")))
	}

	baseWhere := "AND a.status != 'cancelled'"
	if len(statuses) > 0 {
		baseWhere = ""
	}

	query := `
		SELECT a.id, a.starts_at, a.ends_at, a.status, a.price_at_booking,
		       r.name AS resource_name,
		       s.name AS service_name,
		       COALESCE(c.name, c.phone_number) AS customer_name
		FROM appointments a
		JOIN resources r ON r.id = a.resource_id
		JOIN services  s ON s.id = a.service_id
		JOIN customers c ON c.id = a.customer_id
		WHERE a.tenant_id = $1
		  AND a.starts_at >= $2 AND a.starts_at < $3
		  ` + baseWhere

	if len(extraClauses) > 0 {
		query += "\n		  AND " + strings.Join(extraClauses, "\n		  AND ")
	}
	query += "\n		ORDER BY a.starts_at ASC"

	rows, err := h.DB.Primary().QueryContext(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch appointments"})
		return
	}
	defer rows.Close()

	var result []Response
	for rows.Next() {
		var r Response
		var priceAtBooking float64
		if err := rows.Scan(
			&r.ID, &r.StartsAt, &r.EndsAt, &r.Status, &priceAtBooking,
			&r.ResourceName, &r.ServiceName, &r.CustomerName,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch appointments"})
			return
		}
		r.PriceAtBooking = priceAtBooking
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch appointments"})
		return
	}
	if result == nil {
		result = []Response{}
	}

	c.JSON(http.StatusOK, result)
}
