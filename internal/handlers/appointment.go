package handlers

import (
	"appointments/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AppointmentHandler struct {
	DB *gorm.DB
}

func NewAppointmentHandler(db *gorm.DB) *AppointmentHandler {
	return &AppointmentHandler{DB: db}
}

func (h *AppointmentHandler) GetAppointments(c *gin.Context) {
	var appointments []models.Appointment

	if err := h.DB.Find(&appointments).Error; err != nil {
		c.JSON(500, gin.H{"error": "Error fetching appointments"})
		return
	}

	c.JSON(200, appointments)
}
