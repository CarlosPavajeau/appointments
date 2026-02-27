package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type WhatsAppMessageReq struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             *TextObject `json:"text,omitempty"`
}

type TextObject struct {
	Body string `json:"body"`
}

func SendWhatsAppMessage(to string, messageBody string) error {
	apiURL := "https://graph.facebook.com/v18.0/" + os.Getenv("PHONE_NUMBER_ID") + "/messages"
	token := os.Getenv("WHATSAPP_TOKEN")

	payload := WhatsAppMessageReq{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
		Text: &TextObject{
			Body: messageBody,
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling json: %v", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creando request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando request a Meta: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error api meta (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
