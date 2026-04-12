package admin_reject_tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
	"wappiz/pkg/mailer"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	emailSendTimeout = 30 * time.Second
	emailSubject     = "Hemos rechazado tu solicitud"
)

type Request struct {
	Reason string `json:"reason" binding:"required"`
}

type Handler struct {
	DB     db.Database
	Mailer mailer.Mailer
}

func (h *Handler) Method() string {
	return http.MethodPost
}

func (h *Handler) Path() string {
	return "/v1/admin/activations/:id/reject"
}

func (h *Handler) Handle(c *gin.Context) {
	tenantID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}

	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	tenant, err := db.Query.FindTenantByID(ctx, h.DB.Primary(), tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject tenant"})
		return
	}

	waConfig, err := db.Query.FindTenantWhatsappConfig(ctx, h.DB.Primary(), tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("[admin] whatsapp config missing", "tenant_id", tenantID)
			c.JSON(http.StatusNotFound, gin.H{"error": "whatsapp config not found"})
			return
		}

		logger.Warn("[admin] fetch whatsapp config", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load whatsapp config"})
		return
	}

	if err := db.Query.RejectTenantActivation(ctx, h.DB.Primary(), db.RejectTenantActivationParams{
		RejectReason: sql.NullString{String: req.Reason, Valid: req.Reason != ""},
		TenantID:     tenant.ID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject tenant"})
		return
	}

	if waConfig.ActivationContactEmail.Valid {
		scheduleRejectionEmail(h.Mailer, waConfig.ActivationContactEmail.String, tenant.Name, req.Reason)
	}

	c.JSON(http.StatusOK, gin.H{"message": "tenant rejected successfully"})
}

func scheduleRejectionEmail(m mailer.Mailer, to, tenantName, reason string) {
	body := buildRejectEmail(tenantName, reason)

	go func(to, body string) {
		mailCtx, cancel := context.WithTimeout(context.Background(), emailSendTimeout)
		defer cancel()

		if err := m.Send(mailCtx, mailer.Email{
			To:      to,
			Subject: emailSubject,
			Body:    body,
		}); err != nil {
			logger.Warn("[admin] rejection notification email",
				"err", err)
		}
	}(to, body)
}

func buildRejectEmail(tenantName, reason string) string {
	safeName := html.EscapeString(tenantName)
	safeReason := html.EscapeString(reason)

	return fmt.Sprintf(`
		<h2>Hemo rechazado tu solicitud</h2>
		<p>Hola <strong>%s</strong>,</p>
		<p>Desafortunadamente hemos rechazado tu solicitud.</p>
		<h3>Razón:<br>%s</h3>
	`, safeName, safeReason)
}
