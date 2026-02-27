package services

import (
	"fmt"
	"log"

	"appointments/internal/models"

	"gorm.io/gorm"
)

func ProcessConversation(db *gorm.DB, msg struct {
	From string `json:"from"`
	ID   string `json:"id"`
	Type string `json:"type"`
	Text struct {
		Body string `json:"body"`
	} `json:"text"`
}) {
	var conversation models.Conversation

	result := db.FirstOrCreate(&conversation, models.Conversation{Phone: msg.From})
	if result.Error != nil {
		log.Println("Error DB:", result.Error)
		return
	}

	fmt.Printf("Procesando mensaje de %s. Estado actual: %s\n", conversation.Phone, conversation.CurrentStep)

	switch conversation.CurrentStep {
	case "NEW":
		log.Printf("Nuevo usuario: %s", msg.From)

		err := SendWhatsAppMessage(msg.From, "¡Hola! 👋 Bienvenido a *MiBarbería*. \n\nPor favor escribe tu nombre para empezar.")
		if err != nil {
			log.Printf("ERROR enviando mensaje: %v", err)
			return
		}

		conversation.CurrentStep = "WAITING_NAME"
		db.Save(&conversation)

	case "WAITING_NAME":
		nombre := msg.Text.Body

		responseMsg := fmt.Sprintf("Un gusto, %s. 📅 ¿Para qué fecha deseas la cita? (Ej: Mañana 3pm)", nombre)
		err := SendWhatsAppMessage(msg.From, responseMsg)
		if err != nil {
			log.Printf("ERROR enviando mensaje: %v", err)
		}

		conversation.CurrentStep = "WAITING_DATE"
		db.Save(&conversation)

	default:
		conversation.CurrentStep = "NEW"
		db.Save(&conversation)
	}
}
