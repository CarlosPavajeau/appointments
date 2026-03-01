package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client interface {
	SendText(ctx context.Context, to, phoneNumberID, accessToken, body string) error
	SendButtons(ctx context.Context, to, phoneNumberID, accessToken, body string, buttons []Button) error
	SendList(ctx context.Context, to, phoneNumberID, accessToken, body string, sections []Section) error
}

type httpClient struct {
	baseURL    string
	apiVersion string
	http       *http.Client
}

func NewClient(baseURL, apiVersion string) Client {
	return &httpClient{
		baseURL:    baseURL,
		apiVersion: apiVersion,
		http:       &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *httpClient) SendText(ctx context.Context, to, phoneNumberID, accessToken, body string) error {
	req := SendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "text",
		Text:             &OutText{Body: body},
	}
	return c.send(ctx, phoneNumberID, accessToken, req)
}

func (c *httpClient) SendButtons(ctx context.Context, to, phoneNumberID, accessToken, body string, buttons []Button) error {
	req := SendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "interactive",
		Interactive: &OutInteractive{
			Type:   "button",
			Body:   InteractiveBody{Text: body},
			Action: ButtonAction{Buttons: buttons},
		},
	}
	return c.send(ctx, phoneNumberID, accessToken, req)
}

func (c *httpClient) SendList(ctx context.Context, to, phoneNumberID, accessToken, body string, sections []Section) error {
	req := SendMessageRequest{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             "interactive",
		Interactive: &OutInteractive{
			Type: "list",
			Body: InteractiveBody{Text: body},
			Action: ListAction{
				ButtonText: "Ver opciones",
				Sections:   sections,
			},
		},
	}
	return c.send(ctx, phoneNumberID, accessToken, req)
}

func (c *httpClient) send(ctx context.Context, phoneNumberID, accessToken string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, phoneNumberID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("whatsapp api error %d: %s", resp.StatusCode, respBody)
	}

	return nil
}
