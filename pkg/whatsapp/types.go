package whatsapp

// WebhookPayload is the top-level structure of a WhatsApp Cloud API webhook
// notification. It contains one or more entries, each holding a list of
// changes that occurred on the associated WhatsApp Business Account.
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

// Entry represents a single WhatsApp Business Account within a webhook payload.
type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

// Change describes a single event that occurred, identified by Field
// (e.g. "messages") and its associated Value.
type Change struct {
	Value ChangeValue `json:"value"`
	Field string      `json:"field"`
}

// ChangeValue holds the content of a webhook change, including any inbound
// messages and delivery status updates.
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

// Message represents a single inbound message received from a WhatsApp user.
// Text is set for plain-text messages; Interactive is set for button or list
// replies.
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

// SendMessageRequest is the JSON body sent to the Cloud API messages endpoint.
// Only one of Text or Interactive should be set, depending on Type.
type SendMessageRequest struct {
	MessagingProduct string          `json:"messaging_product"`
	RecipientType    string          `json:"recipient_type"`
	To               string          `json:"to"`
	Type             string          `json:"type"`
	Text             *OutText        `json:"text,omitempty"`
	Interactive      *OutInteractive `json:"interactive,omitempty"`
}

type OutText struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

// OutInteractive is the interactive field of an outbound message.
// Action must be either a [ButtonAction] or a [ListAction] depending on Type.
type OutInteractive struct {
	Type   string          `json:"type"`
	Body   InteractiveBody `json:"body"`
	Action interface{}     `json:"action"` // ButtonAction | ListAction
}

type InteractiveBody struct {
	Text string `json:"text"`
}

// Buttons (máx 3)
type ButtonAction struct {
	Buttons []Button `json:"buttons"`
}

type Button struct {
	Type  string      `json:"type"`
	Reply ButtonReply `json:"reply"`
}

// List (máx 10 per section)
type ListAction struct {
	ButtonText string    `json:"button"`
	Sections   []Section `json:"sections"`
}

type Section struct {
	Title string    `json:"title"`
	Rows  []ListRow `json:"rows"`
}

type ListRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}
