package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"appointments/internal/handlers"
	"appointments/internal/models"
)

func main() {
	dsn := os.Getenv("DB_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Fallo al conectar a la DB:", err)
	}

	if err := db.AutoMigrate(&models.Conversation{}, &models.Appointment{}); err != nil {
		log.Fatal("Error migrando base de datos: ", err)
	}

	r := gin.Default()

	webhookHandler := handlers.NewWebhookHandler(db)

	r.GET("/webhook", webhookHandler.VerifyToken)
	r.POST("/webhook", webhookHandler.ReceiveMessage)

	r.Run(":8080")
}
