package onboarding_step_whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"wappiz/pkg/db"
	"wappiz/pkg/jwt"
	"wappiz/pkg/mailer"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const stepWhatsApp int32 = 4

type Request struct {
	ContactEmail string `json:"contactEmail" binding:"required,email"`
	Notes        string `json:"notes"`
}

type Handler struct {
	DB         db.Database
	Mailer     mailer.Mailer
	AdminEmail string
}

func (h *Handler) Method() string { return http.MethodPost }
func (h *Handler) Path() string   { return "/v1/onboarding/step/4" }

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := jwt.TenantIDFromContext(c)

	progress, err := db.Query.FindOnboardingProgressByTenant(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if progress.CurrentStep < stepWhatsApp {
		c.JSON(http.StatusForbidden, gin.H{"error": "step not available yet"})
		return
	}

	tenant, err := db.Query.FindTenantByID(c.Request.Context(), h.DB.Primary(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if err := db.Query.InsertTenantWhatsappConfig(c.Request.Context(), h.DB.Primary(), db.InsertTenantWhatsappConfigParams{
		ID:                     uuid.New(),
		TenantID:               tenantID,
		ActivationContactEmail: sql.NullString{String: req.ContactEmail, Valid: true},
		ActivationNotes:        sql.NullString{String: req.Notes, Valid: req.Notes != ""},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if err := db.Query.CompleteOnboardingProgress(c.Request.Context(), h.DB.Primary(), tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	bgCtx := context.WithoutCancel(c.Request.Context())
	tenantName := tenant.Name
	contactEmail := req.ContactEmail
	notes := req.Notes
	adminEmail := h.AdminEmail

	go func() {
		if err := h.Mailer.Send(bgCtx, mailer.Email{
			To:      contactEmail,
			Subject: "✂️ Estamos configurando tu WhatsApp",
			Body:    buildOwnerRequestEmail(tenantName),
		}); err != nil {
			log.Printf("onboarding: send owner email tenant_id=%s email=%s err=%v", tenantID, contactEmail, err)
		}
	}()

	go func() {
		if err := h.Mailer.Send(bgCtx, mailer.Email{
			To:      adminEmail,
			Subject: fmt.Sprintf("🔔 Nueva activación pendiente: %s", tenantName),
			Body:    buildAdminNotificationEmail(tenantName, contactEmail, notes),
		}); err != nil {
			log.Printf("onboarding: send admin email tenant_id=%s email=%s err=%v", tenantID, contactEmail, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"redirect": "/dashboard"})
}

func buildOwnerRequestEmail(tenantName string) string {
	return fmt.Sprintf(`
		<h2>¡Hola!</h2>
		<p>Recibimos tu solicitud para activar el WhatsApp de <strong>%s</strong>.</p>
		<p>Nuestro equipo está trabajando en ello.</p>
		<p><strong>Tiempo estimado: 2 horas hábiles.</strong></p>
		<p>Mientras esperas puedes personalizar tu panel.</p>
	`, tenantName)
}

func buildAdminNotificationEmail(tenantName, contactEmail, notes string) string {
	return fmt.Sprintf(`
		<h2>Nueva activación pendiente</h2>
		<p><strong>Barbería:</strong> %s</p>
		<p><strong>Correo:</strong> %s</p>
		<p><strong>Notas:</strong> %s</p>
	`, tenantName, contactEmail, notes)
}
