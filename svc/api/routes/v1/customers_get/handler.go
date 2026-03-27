package customers_get

import (
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID          uuid.UUID `json:"id"`
	PhoneNumber string    `json:"phoneNumber"`
	Name        *string   `json:"name"`
	DisplayName string    `json:"displayName"`
	IsBlocked   bool      `json:"isBlocked"`
}

type Handler struct {
	DB db.Database
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/customers/:id" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	customer, err := db.Query.FindCustomerByID(c.Request.Context(), h.DB.Primary(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}
	if customer.TenantID != tenantID {
		c.JSON(http.StatusNotFound, gin.H{"error": "customer not found"})
		return
	}

	var name *string
	if customer.Name.Valid {
		name = &customer.Name.String
	}
	displayName := customer.PhoneNumber
	if customer.Name.Valid && customer.Name.String != "" {
		displayName = customer.Name.String
	}

	c.JSON(http.StatusOK, Response{
		ID:          customer.ID,
		PhoneNumber: customer.PhoneNumber,
		Name:        name,
		DisplayName: displayName,
		IsBlocked:   customer.IsBlocked,
	})
}
