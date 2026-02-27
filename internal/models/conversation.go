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
