package webhooks_process_webhook

import (
	"net/http"
	"wappiz/internal/services/webhook_processor"
	"wappiz/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Processor webhook_processor.Service
}

func (h *Handler) Method() string {
	return http.MethodPost
}

func (h *Handler) Path() string {
	return "/webhook"
}

func (h *Handler) Handle(c *gin.Context) {
	var req webhook_processor.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("webhook: failed to parse payload",
			"err", err)
		c.Status(http.StatusOK)
		return
	}

	if req.Object != "whatsapp_business_account" {
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)

	h.Processor.Enqueue(req)
}
