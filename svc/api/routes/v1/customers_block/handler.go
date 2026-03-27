package customers_block

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

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/customers/:id/block" }

func (h *Handler) Handle(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid customer id"})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	if err := db.Query.BlockCustomer(c.Request.Context(), h.DB.Primary(), db.BlockCustomerParams{
		ID:       id,
		TenantID: tenantID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to block customer"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "customer blocked"})
}
