package plans_list_active

import (
	"net/http"
	"wappiz/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Response struct {
	ID          uuid.UUID `json:"id"`
	ExternalID  string    `json:"external_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int32     `json:"price"`
	Currency    string    `json:"currency"`
	Interval    string    `json:"interval"`
}

type Handler struct {
	DB          db.Database
	Environment string
}

func (h *Handler) Method() string { return http.MethodGet }
func (h *Handler) Path() string   { return "/v1/plans" }

func (h *Handler) Handle(c *gin.Context) {
	plans, err := db.Query.ListActivePlans(c.Request.Context(), h.DB.Primary(), h.Environment)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	response := make([]Response, len(plans))
	for i, plan := range plans {
		response[i] = Response{
			ID:          plan.ID,
			ExternalID:  plan.ExternalID,
			Name:        plan.Name,
			Description: plan.Description.String,
			Price:       plan.Price,
			Currency:    plan.Currency,
			Interval:    plan.Interval.String,
		}
	}

	c.JSON(http.StatusOK, response)
}
