package users

import (
	"net/http"
	"wappiz/internal/shared/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	useCases *UseCases
}

func NewHandler(uc *UseCases) *Handler {
	return &Handler{useCases: uc}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	protected := r.Group("/api/v1/users")
	protected.Use(jwt.AuthMiddleware())
	{
		protected.GET("/me", h.GetMe)
	}
}

func (h *Handler) GetMe(c *gin.Context) {
	userID, err := uuid.Parse(jwt.UserIDFromContext(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := h.useCases.repo.FindByID(c.Request.Context(), userID)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Failed to retrieve user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        user.ID,
		"tenantId":  user.TenantID,
		"email":     user.Email,
		"role":      user.Role,
		"createdAt": user.CreatedAt,
	})
}
