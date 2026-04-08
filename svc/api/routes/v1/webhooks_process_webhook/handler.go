package webhooks_process_webhook

import (
	"context"
	"database/sql"
	"net/http"
	"time"
	"wappiz/internal/services/state_machine"
	"wappiz/pkg/crypto"
	"wappiz/pkg/db"
	"wappiz/pkg/logger"

	"github.com/gin-gonic/gin"
)

type Request struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Value ChangeValue `json:"value"`
	Field string      `json:"field"`
}

type ChangeValue struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Messages         []Message `json:"messages"`
	Statuses         []Status  `json:"statuses"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Message struct {
	ID          string       `json:"id"`
	From        string       `json:"from"`
	Timestamp   string       `json:"timestamp"`
	Type        string       `json:"type"`
	Text        *TextMessage `json:"text,omitempty"`
	Interactive *Interactive `json:"interactive,omitempty"`
}

type TextMessage struct {
	Body string `json:"body"`
}

type Interactive struct {
	Type        string       `json:"type"`
	ButtonReply *ButtonReply `json:"button_reply,omitempty"`
	ListReply   *ListReply   `json:"list_reply,omitempty"`
}

type ButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ListReply struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Status struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Timestamp   string `json:"timestamp"`
	RecipientID string `json:"recipient_id"`
}

type Handler struct {
	DB            db.Database
	StateMachine  state_machine.StateMachineService
	EncryptionKey []byte
}

func (h *Handler) Method() string {
	return http.MethodPost
}

func (h *Handler) Path() string {
	return "/webhook"
}

func (h *Handler) Handle(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("webhook: failed to parse payload: %v", err)
		c.Status(http.StatusOK)
		return
	}

	if req.Object != "whatsapp_business_account" {
		c.Status(http.StatusOK)
		return
	}

	c.Status(http.StatusOK)

	go h.processPayload(req)
}

func (h *Handler) processPayload(req Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, entry := range req.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}

			phoneNumberID := change.Value.Metadata.PhoneNumberID
			waConfig, err := db.Query.FindTenantWhatsappConfigByPhoneNumberID(ctx, h.DB.Primary(), sql.NullString{
				String: phoneNumberID,
				Valid:  true,
			})

			if err != nil {
				logger.Warn("webhook: unknown phone_number_id",
					"phone_number_id", phoneNumberID,
					"err", err)
				continue
			}

			for _, msg := range change.Value.Messages {
				incoming, err := h.buildIncomingMessage(msg, change.Value.Metadata, waConfig)
				if err != nil {
					logger.Warn("webhook: failed to build message",
						"from", msg.From,
						"err", err)
					continue
				}

				if err := h.StateMachine.Process(ctx, *incoming); err != nil {
					logger.Warn("webhook: error processing message from",
						"from", msg.From,
						"err", err)
				}
			}

			for _, status := range change.Value.Statuses {
				logger.Info("webhook: received status update",
					"id", status.ID,
					"status", status.Status,
					"recipient_id", status.RecipientID)
			}
		}
	}
}

func (h *Handler) buildIncomingMessage(msg Message, metadata Metadata, waConfig db.FindTenantWhatsappConfigByPhoneNumberIDRow) (*state_machine.IncomingMessage, error) {
	accessToken, err := crypto.Decrypt(waConfig.AccessToken.String, h.EncryptionKey)
	if err != nil {
		return nil, err
	}

	incoming := &state_machine.IncomingMessage{
		TenantID:         waConfig.TenantID,
		WhatsappConfigID: waConfig.ID,
		PhoneNumberID:    metadata.PhoneNumberID,
		AccessToken:      accessToken,
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
				incoming.InteractiveID = new(msg.Interactive.ButtonReply.ID)
				incoming.Body = msg.Interactive.ButtonReply.Title
			}
		case "list_reply":
			if msg.Interactive.ListReply != nil {
				incoming.InteractiveID = new(msg.Interactive.ListReply.ID)
				incoming.Body = msg.Interactive.ListReply.Title
			}
		}

	default:
		// Unsupported message types: audio, image, document, video, sticker, location, contacts, etc.
		// We can log them for now, but we won't process them until we have a use case for it.
		logger.Warn("webhook: unsupported message",
			"type", msg.Type,
			"from", msg.From)
	}

	return incoming, nil
}
