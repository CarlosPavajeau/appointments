package scheduling

import (
	"context"
	"log"
	"net/http"
	"time"

	"appointments/internal/features/tenants"
	"appointments/internal/platform/whatsapp"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	machine     *StateMachine
	tenantRepo  tenants.Repository
	verifyToken string
}

func NewHandler(machine *StateMachine, tenantRepo tenants.Repository, verifyToken string) *Handler {
	return &Handler{
		machine:     machine,
		tenantRepo:  tenantRepo,
		verifyToken: verifyToken,
	}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.GET("/webhook", h.Verify)
	r.POST("/webhook", h.Handle)
}

func (h *Handler) Verify(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode == "subscribe" && token == h.verifyToken {
		c.String(http.StatusOK, challenge)
		return
	}

	c.Status(http.StatusForbidden)
}

func (h *Handler) Handle(c *gin.Context) {
	var payload whatsapp.WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("webhook: failed to parse payload: %v", err)
		c.Status(http.StatusOK)
		return
	}

	if payload.Object != "whatsapp_business_account" {
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)

	go h.processPayload(payload)
}

func (h *Handler) processPayload(payload whatsapp.WebhookPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			phoneNumberID := change.Value.Metadata.PhoneNumberID
			waConfig, tenant, err := h.tenantRepo.FindWhatsappConfigByPhoneNumberID(ctx, phoneNumberID)
			if err != nil {
				log.Printf("webhook: unknown phone_number_id %s: %v", phoneNumberID, err)
				continue
			}

			for _, msg := range change.Value.Messages {
				incoming, err := h.buildIncomingMessage(msg, change.Value.Metadata, waConfig, tenant)
				if err != nil {
					log.Printf("webhook: failed to build message from %s: %v", msg.From, err)
					continue
				}

				if err := h.machine.Process(ctx, *incoming); err != nil {
					log.Printf("webhook: error processing message from %s: %v", msg.From, err)
				}
			}

			for _, status := range change.Value.Statuses {
				log.Printf("webhook: message %s status=%s recipient=%s",
					status.ID, status.Status, status.RecipientID)
			}
		}
	}
}

func (h *Handler) buildIncomingMessage(
	msg whatsapp.Message,
	metadata whatsapp.Metadata,
	waConfig *tenants.WhatsappConfig,
	tenant *tenants.Tenant,
) (*IncomingMessage, error) {

	incoming := &IncomingMessage{
		TenantID:         tenant.ID,
		WhatsappConfigID: waConfig.ID,
		PhoneNumberID:    metadata.PhoneNumberID,
		AccessToken:      waConfig.AccessToken,
		From:             msg.From,
		ReceivedAt:       time.Now(),
	}

	switch msg.Type {
	case "text":
		if msg.Text != nil {
			incoming.Body = msg.Text.Body
		}

	case "interactive":
		if msg.Interactive == nil {
			break
		}
		switch msg.Interactive.Type {
		case "button_reply":
			if msg.Interactive.ButtonReply != nil {
				id := msg.Interactive.ButtonReply.ID
				incoming.InteractiveID = &id
				incoming.Body = msg.Interactive.ButtonReply.Title
			}
		case "list_reply":
			if msg.Interactive.ListReply != nil {
				id := msg.Interactive.ListReply.ID
				incoming.InteractiveID = &id
				incoming.Body = msg.Interactive.ListReply.Title
			}
		}

	default:
		// Tunsupported message types: audio, image, document, video, sticker, location, contacts, etc.
		// We can log them for now, but we won't process them until we have a use case for it.
		log.Printf("webhook: unsupported message type %s from %s", msg.Type, msg.From)
	}

	return incoming, nil
}
