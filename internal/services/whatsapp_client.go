package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// ReplyButton represents a single interactive reply button.
type ReplyButton struct {
	ID    string
	Title string
}

// ---- Outgoing message payload types ----

type textMessageReq struct {
	MessagingProduct string       `json:"messaging_product"`
	To               string       `json:"to"`
	Type             string       `json:"type"`
	Text             *messageText `json:"text,omitempty"`
}

// messageText is the "text" object in a plain text message.
// WhatsApp schema: {"text": {"body": "..."}}
type messageText struct {
	Body string `json:"body"`
}

type interactiveMessageReq struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Interactive      interactive `json:"interactive"`
}

type interactive struct {
	Type   string          `json:"type"`
	Body   interactiveBody `json:"body"`
	Action buttonAction    `json:"action"`
}

// interactiveBody is the "body" object inside an interactive message.
// WhatsApp schema: {"interactive": {"body": {"text": "..."}, ...}}
type interactiveBody struct {
	Text string `json:"text"`
}

type buttonAction struct {
	Buttons []actionButton `json:"buttons"`
}

type actionButton struct {
	Type  string      `json:"type"`
	Reply buttonReply `json:"reply"`
}

type buttonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ---- Client initialisation ----

// httpClient is shared across all calls to reuse TCP connections via keep-alive.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// waConfig holds values read once from the environment at first use.
var (
	waOnce   sync.Once
	waAPIURL string
	waToken  string
)

func initWAConfig() {
	waOnce.Do(func() {
		phoneID := os.Getenv("PHONE_NUMBER_ID")
		waAPIURL = fmt.Sprintf("https://graph.facebook.com/v18.0/%s/messages", phoneID)
		waToken = os.Getenv("WHATSAPP_TOKEN")
	})
}

// ---- Public send functions ----

func SendWhatsAppMessage(to, body string) error {
	initWAConfig()
	return doPost(textMessageReq{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
		Text:             &messageText{Body: body},
	})
}

func SendWhatsAppButtons(to, bodyText string, buttons []ReplyButton) error {
	initWAConfig()
	actionButtons := make([]actionButton, len(buttons))
	for i, b := range buttons {
		actionButtons[i] = actionButton{
			Type:  "reply",
			Reply: buttonReply{ID: b.ID, Title: b.Title},
		}
	}
	return doPost(interactiveMessageReq{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "interactive",
		Interactive: interactive{
			Type:   "button",
			Body:   interactiveBody{Text: bodyText},
			Action: buttonAction{Buttons: actionButtons},
		},
	})
}

// doPost marshals payload, sends it to the WhatsApp API, and handles errors.
func doPost(payload any) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, waAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+waToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("meta api error (status %d): %s", resp.StatusCode, bodyBytes)
	}

	// Drain body so the underlying TCP connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
