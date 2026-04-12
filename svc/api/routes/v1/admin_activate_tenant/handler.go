package admin_activate_tenant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"time"
	"wappiz/pkg/crypto"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
	"wappiz/pkg/mailer"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	activationEmailSendTimeout = 30 * time.Second
	activationEmailSubject     = "¡Tu barbería ya puede recibir citas!"
)

type Request struct {
	PhoneNumberID      string `json:"phoneNumberId"      binding:"required"`
	DisplayPhoneNumber string `json:"displayPhoneNumber" binding:"required"`
	WABAID             string `json:"wabaId"             binding:"required"`
	AccessToken        string `json:"accessToken"        binding:"required"`
}

type Handler struct {
	DB     db.Database
	Mailer mailer.Mailer
	Crypto *crypto.Service
}

func (h *Handler) Method() string {
	return http.MethodPost
}

func (h *Handler) Path() string {
	return "/v1/admin/activations/:id/activate"
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

	tx, err := h.DB.Primary().Begin(ctx)
	if err != nil {
		logger.Warn("[admin] begin transaction for activation", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}
	defer tx.Rollback()

	tenant, err := db.Query.FindTenantByID(ctx, tx, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
			return
		}
		logger.Warn("[admin] find tenant for activation", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}

	accessToken, err := h.Crypto.Encrypt(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := db.Query.ActivateTenantWhatsappConfig(ctx, tx, db.ActivateTenantWhatsappConfigParams{
		WabaID:             sql.NullString{String: req.WABAID, Valid: req.WABAID != ""},
		PhoneNumberID:      sql.NullString{String: req.PhoneNumberID, Valid: req.PhoneNumberID != ""},
		DisplayPhoneNumber: sql.NullString{String: req.DisplayPhoneNumber, Valid: req.DisplayPhoneNumber != ""},
		AccessToken:        sql.NullString{String: accessToken, Valid: accessToken != ""},
		TenantID:           tenantID,
	}); err != nil {
		logger.Warn("[admin] activate whatsapp config", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}

	waConfig, err := db.Query.FindTenantWhatsappConfig(ctx, tx, tenantID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("[admin] whatsapp config missing after activation", "tenant_id", tenantID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "whatsapp config not found after activation"})
			return
		}
		logger.Warn("[admin] fetch whatsapp config after activation", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load whatsapp config"})
		return
	}

	if err := tx.Commit(); err != nil {
		logger.Warn("[admin] commit activation transaction", "tenant_id", tenantID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}

	if waConfig.ActivationContactEmail.Valid {
		scheduleActivationEmail(h.Mailer, tenantID, waConfig.ActivationContactEmail.String, tenant.Name, req.DisplayPhoneNumber)
	}

	c.JSON(http.StatusOK, gin.H{"message": "tenant activated"})
}

// scheduleActivationEmail sends the tenant activation notification asynchronously
// (non-blocking for the HTTP handler). Errors are logged only.
func scheduleActivationEmail(m mailer.Mailer, tenantID uuid.UUID, to, tenantName, displayPhoneNumber string) {
	body := buildActivationEmail(tenantName, displayPhoneNumber)

	go func(tenantID uuid.UUID, to, body string) {
		mailCtx, cancel := context.WithTimeout(context.Background(), activationEmailSendTimeout)
		defer cancel()

		if err := m.Send(mailCtx, mailer.Email{
			To:      to,
			Subject: activationEmailSubject,
			Body:    body,
		}); err != nil {
			logger.Warn("[admin] activation notification email",
				"tenant_id", tenantID,
				"to", to,
				"err", err)
		}
	}(tenantID, to, body)
}

func buildActivationEmail(tenantName, phoneNumber string) string {
	waLink := "https://wa.me/" + sanitizePhone(phoneNumber)
	safeName := html.EscapeString(tenantName)
	safePhone := html.EscapeString(phoneNumber)
	safeLink := html.EscapeString(waLink)
	return fmt.Sprintf(`
		<h2>🎉 ¡Tu barbería ya puede recibir citas!</h2>
		<p>Hola <strong>%s</strong>,</p>
		<p>Tu número de WhatsApp ya está listo.</p>
		<h3>📱 Número de tu barbería:<br>%s</h3>
		<p>Dale este número a tus clientes o comparte el enlace directo:</p>
		<p><a href="%s">%s</a></p>
	`, safeName, safePhone, safeLink, safeLink)
}

func sanitizePhone(phone string) string {
	result := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			result += string(ch)
		}
	}
	return result
}
