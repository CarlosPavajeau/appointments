package handlers

import (
	"net/http"
	"os"

	"appointments/internal/models"
	"appointments/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	DB *gorm.DB
}

func NewWebhookHandler(db *gorm.DB) *WebhookHandler {
	return &WebhookHandler{DB: db}
}

func (h *WebhookHandler) VerifyToken(c *gin.Context) {
	verifyToken := os.Getenv("VERIFY_TOKEN")
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == verifyToken {
		c.String(http.StatusOK, challenge)
	} else {
		c.String(http.StatusForbidden, "Forbidden")
	}
}

func (h *WebhookHandler) ReceiveMessage(c *gin.Context) {
	var payload models.WebhookPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)

	go func(p models.WebhookPayload) {
		for _, entry := range p.Entry {
			for _, change := range entry.Changes {
				if len(change.Value.Messages) > 0 {
					msg := change.Value.Messages[0]
					services.ProcessConversation(h.DB, msg)
				}
			}
		}
	}(payload)
}
