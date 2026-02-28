package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"appointments/internal/models"

	"gorm.io/gorm"
)

const (
	AppointmentDuration = 1 * time.Hour
	TimeZone            = "America/Bogota"
)

// AppointmentStatus represents the lifecycle state of an appointment.
type AppointmentStatus string

const StatusConfirmed AppointmentStatus = "CONFIRMED"

// Sentinel errors allow callers to distinguish failure modes with errors.Is.
var (
	ErrInvalidFormat = errors.New("formato inválido")
	ErrDateInThePast = errors.New("la fecha ya pasó")
)

// bogotaLoc is loaded once at startup; panics if the timezone is unavailable
// (which would indicate a broken Go installation).
var bogotaLoc = mustLoadLocation(TimeZone)

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("failed to load timezone %q: %v", name, err))
	}
	return loc
}

func ParseAppointmentTime(input string) (time.Time, error) {
	const layout = "02/01 15:04"

	parsed, err := time.ParseInLocation(layout, strings.TrimSpace(input), bogotaLoc)
	if err != nil {
		return time.Time{}, ErrInvalidFormat
	}

	now := time.Now().In(bogotaLoc)
	finalTime := time.Date(now.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), 0, 0, bogotaLoc)

	if finalTime.Before(now) {
		return time.Time{}, ErrDateInThePast
	}

	return finalTime, nil
}

func CheckAvailability(db *gorm.DB, startTime time.Time) (bool, error) {
	endTime := startTime.Add(AppointmentDuration)
	var count int64

	err := db.Model(&models.Appointment{}).
		Where("status = ? AND start_time < ? AND end_time > ?", StatusConfirmed, endTime, startTime).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check availability: %w", err)
	}

	return count == 0, nil
}

func GetLatestAppointment(db *gorm.DB, phone string) (*models.Appointment, error) {
	var appt models.Appointment
	err := db.Where("client_phone = ?", phone).
		Order("created_at DESC").
		First(&appt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest appointment: %w", err)
	}
	return &appt, nil
}

func CreateAppointment(db *gorm.DB, phone, name string, startTime time.Time) error {
	appointment := models.Appointment{
		ClientPhone: phone,
		ClientName:  name,
		StartTime:   startTime,
		EndTime:     startTime.Add(AppointmentDuration),
		Status:      string(StatusConfirmed),
	}
	if err := db.Create(&appointment).Error; err != nil {
		return fmt.Errorf("create appointment: %w", err)
	}
	return nil
}
