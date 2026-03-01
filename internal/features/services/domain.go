package services

import (
	"time"

	"github.com/google/uuid"
)

type Service struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Name            string
	Description     string
	DurationMinutes int
	BufferMinutes   int
	Price           float64
	IsActive        bool
	SortOrder       int
	CreatedAt       time.Time
}

func (s *Service) TotalDuration() int {
	return s.DurationMinutes + s.BufferMinutes
}

func (s *Service) Validate() error {
	if s.Name == "" {
		return ErrNameRequired
	}
	if s.DurationMinutes <= 0 {
		return ErrInvalidDuration
	}
	if s.Price < 0 {
		return ErrInvalidPrice
	}
	if s.BufferMinutes < 0 {
		return ErrInvalidBuffer
	}
	return nil
}

var (
	ErrNameRequired    = serviceError("name is required")
	ErrInvalidDuration = serviceError("duration must be greater than 0")
	ErrInvalidPrice    = serviceError("price cannot be negative")
	ErrInvalidBuffer   = serviceError("buffer cannot be negative")
)

type serviceError string

func (e serviceError) Error() string { return string(e) }
