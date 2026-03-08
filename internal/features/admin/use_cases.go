package admin

import (
	"context"
	"fmt"

	"appointments/internal/features/tenants"
	"appointments/internal/platform/mailer"

	"github.com/google/uuid"
)

// TenantService defines the tenant operations needed by admin.
type TenantService interface {
	FindByID(ctx context.Context, id uuid.UUID) (*tenants.Tenant, error)
	ActivateWhatsappConfig(ctx context.Context, input tenants.ActivateWhatsappConfigInput) error
	FindPendingActivations(ctx context.Context) ([]tenants.PendingActivation, error)
}

type UseCases struct {
	tenantService TenantService
	mailer        mailer.Mailer
}

func NewUseCases(tenantService TenantService, mailer mailer.Mailer) *UseCases {
	return &UseCases{
		tenantService: tenantService,
		mailer:        mailer,
	}
}

// ListActivations returns all tenants pending WhatsApp activation.
func (uc *UseCases) ListActivations(ctx context.Context) ([]Activation, error) {
	pending, err := uc.tenantService.FindPendingActivations(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]Activation, len(pending))
	for i, p := range pending {
		result[i] = Activation{
			TenantID:     p.TenantID,
			TenantName:   p.TenantName,
			ContactEmail: p.ContactEmail,
			Notes:        p.Notes,
			Status:       ActivationStatus(p.Status),
			RequestedAt:  p.RequestedAt,
		}
	}
	return result, nil
}

type ActivateTenantInput struct {
	TenantID           uuid.UUID
	PhoneNumberID      string
	DisplayPhoneNumber string
	WABAID             string
	AccessToken        string
}

// ActivateTenant activates the WhatsApp config for a tenant and notifies the owner.
func (uc *UseCases) ActivateTenant(ctx context.Context, input ActivateTenantInput) error {
	tenant, err := uc.tenantService.FindByID(ctx, input.TenantID)
	if err != nil {
		return fmt.Errorf("find tenant: %w", err)
	}

	if err := uc.tenantService.ActivateWhatsappConfig(ctx, tenants.ActivateWhatsappConfigInput{
		TenantID:           input.TenantID,
		PhoneNumberID:      input.PhoneNumberID,
		DisplayPhoneNumber: input.DisplayPhoneNumber,
		WABAID:             input.WABAID,
		AccessToken:        input.AccessToken,
	}); err != nil {
		return fmt.Errorf("activate whatsapp config: %w", err)
	}

	go uc.mailer.Send(ctx, mailer.Email{
		To:      tenant.Settings.ContactEmail,
		Subject: "🎉 Tu WhatsApp ya está activo — Turnio",
		Body:    buildActivationEmail(tenant.Name, input.DisplayPhoneNumber),
	})

	return nil
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
