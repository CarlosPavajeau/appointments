package services

import (
	"encoding/json"
	"fmt"
	"log"

	"appointments/internal/models"

	"gorm.io/gorm"
)

type TempDataStruct struct {
	DraftName string `json:"draft_name"`
}

func ProcessConversation(db *gorm.DB, msg models.WhatsAppMessage) {
	var conv models.Conversation

	if err := db.FirstOrCreate(&conv, models.Conversation{Phone: msg.From}).Error; err != nil {
		log.Printf("Error DB: %v", err)
		return
	}

	log.Printf("Usuario: %s | Estado: %s | Input: %s", msg.From, conv.CurrentStep, msg.Text.Body)

	switch conv.CurrentStep {
	case "NEW":
		SendWhatsAppMessage(msg.From, "¡Hola! 👋 Bienvenido al sistema de agendamiento.\n\nPor favor, escribe tu *Nombre Completo* para comenzar:")
		updateState(db, &conv, "WAITING_NAME", nil)

	case "WAITING_NAME":
		name := msg.Text.Body
		if len(name) < 3 {
			SendWhatsAppMessage(msg.From, "El nombre es muy corto. Por favor intenta de nuevo:")
			return
		}

		tempData := TempDataStruct{DraftName: name}

		msgRes := fmt.Sprintf("Gracias %s. 🗓️\n\nPor favor escribe la fecha y hora deseada en formato *DD/MM HH:MM*.\nEjemplo: *28/02 15:00* (para el 28 de feb a las 3 PM).", name)
		SendWhatsAppMessage(msg.From, msgRes)

		updateState(db, &conv, "WAITING_DATE", tempData)

	case "WAITING_DATE":
		inputDate := msg.Text.Body

		bookedTime, err := ParseAppointmentTime(inputDate)
		if err != nil {
			SendWhatsAppMessage(msg.From, "⚠️ Formato inválido o fecha pasada.\nUsa el formato: *DD/MM HH:MM* (Ej: 28/02 15:00)")
			return
		}

		isFree, err := CheckAvailability(db, bookedTime)
		if err != nil {
			SendWhatsAppMessage(msg.From, "Error interno verificando agenda. Intenta más tarde.")
			log.Printf("Error DB: %v", err)
			return
		}
		if !isFree {
			SendWhatsAppMessage(msg.From, "❌ Ese horario ya está ocupado.\nPor favor escribe otra hora (Ej: 28/02 16:00):")
			return
		}

		var data TempDataStruct
		if len(conv.TempData) > 0 {
			json.Unmarshal(conv.TempData, &data)
		}

		err = CreateAppointment(db, msg.From, data.DraftName, bookedTime)
		if err != nil {
			SendWhatsAppMessage(msg.From, "No pudimos guardar tu cita. Intenta de nuevo.")
			return
		}

		confirmMsg := fmt.Sprintf("✅ *¡Cita Confirmada!*\n\nCliente: %s\nFecha: %s\n\nTe esperamos.", data.DraftName, bookedTime.Format("02/01/2006 03:04 PM"))
		SendWhatsAppMessage(msg.From, confirmMsg)

		updateState(db, &conv, "NEW", TempDataStruct{}) // Limpiamos datos

	default:
		SendWhatsAppMessage(msg.From, "Sesión reiniciada. Escribe tu nombre para agendar:")
		updateState(db, &conv, "WAITING_NAME", nil)
	}
}

func updateState(db *gorm.DB, conv *models.Conversation, newStep string, data interface{}) {
	conv.CurrentStep = newStep

	if data != nil {
		jsonData, _ := json.Marshal(data)
		conv.TempData = jsonData
	} else {
		conv.TempData = []byte("{}")
	}

	db.Save(conv)
}
