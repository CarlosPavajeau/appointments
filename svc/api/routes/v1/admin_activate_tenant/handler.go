package admin_activate_tenant

import (
	"database/sql"
	"fmt"
	"net/http"
	"wappiz/pkg/crypto"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"
	"wappiz/pkg/mailer"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	accessToken, err := h.Crypto.Encrypt(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = db.Query.ActivateTenantWhatsappConfig(c.Request.Context(), h.DB.Primary(), db.ActivateTenantWhatsappConfigParams{
		WabaID:             sql.NullString{String: req.WABAID},
		PhoneNumberID:      sql.NullString{String: req.PhoneNumberID},
		DisplayPhoneNumber: sql.NullString{String: req.DisplayPhoneNumber},
		AccessToken:        sql.NullString{String: accessToken},
		TenantID:           tenantID,
	})

	if err != nil {
		logger.Warn("[admin] activate whatsapp config", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate tenant"})
		return
	}

	// TODO: Send notification email

	c.JSON(http.StatusOK, gin.H{"message": "tenant activated"})
}

func buildActivationEmail(tenantName, phoneNumber string) string {
	waLink := "https://wa.me/" + sanitizePhone(phoneNumber)
	return fmt.Sprintf(`
		<h2>🎉 ¡Tu barbería ya puede recibir citas!</h2>
		<p>Hola <strong>%s</strong>,</p>
		<p>Tu número de WhatsApp ya está listo.</p>
		<h3>📱 Número de tu barbería:<br>%s</h3>
		<p>Dale este número a tus clientes o comparte el enlace directo:</p>
		<p><a href="%s">%s</a></p>
	`, tenantName, phoneNumber, waLink, waLink)
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
