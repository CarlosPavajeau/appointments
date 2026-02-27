package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Conversation struct {
	gorm.Model
	Phone       string         `gorm:"uniqueIndex;not null" json:"phone"`
	CurrentStep string         `gorm:"default:'NEW'" json:"current_step"`
	TempData    datatypes.JSON `json:"temp_data"`
}

type WebhookPayload struct {
	Entry []struct {
		Changes []struct {
			Value struct {
				Messages []struct {
					From string `json:"from"`
					ID   string `json:"id"`
					Type string `json:"type"`
					Text struct {
						Body string `json:"body"`
					} `json:"text"`
				} `json:"messages"`
			} `json:"value"`
		} `json:"changes"`
	} `json:"entry"`
}
