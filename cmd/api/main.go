package main

import (
	"appointments/internal/database"
	"appointments/internal/handlers"
	"appointments/internal/models"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	db, err := database.InitDB()
	if err != nil {
		log.Fatal("Cannot connect to database", err)
	}

	if err := db.AutoMigrate(&models.Conversation{}, &models.Appointment{}); err != nil {
		log.Fatal("Failed to migrate the database: ", err)
	}

	r := gin.Default()

	webhookHandler := handlers.NewWebhookHandler(db)
	appointmentHandler := handlers.NewAppointmentHandler(db)

	r.GET("/webhook", webhookHandler.VerifyToken)
	r.POST("/webhook", webhookHandler.ReceiveMessage)

	r.GET("/appointments", appointmentHandler.GetAppointments)

	r.Run(":8080")
}
