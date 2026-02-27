package models

type TextObject struct {
	Body string `json:"body"`
}

type WhatsAppMessage struct {
	From      string     `json:"from"`
	ID        string     `json:"id"`
	Timestamp string     `json:"timestamp"`
	Type      string     `json:"type"`
	Text      TextObject `json:"text"`
}

type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string                 `json:"messaging_product"`
				Metadata         map[string]interface{} `json:"metadata"`
				Contacts         []interface{}          `json:"contacts"`
				Messages         []WhatsAppMessage      `json:"messages"` // Aquí usamos el tipo nombrado
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}
