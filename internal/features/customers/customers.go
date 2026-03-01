package customers

import (
	"time"

	"github.com/google/uuid"
)

type Customer struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	PhoneNumber string
	Name        *string
	IsBlocked   bool
	CreatedAt   time.Time
}

func (c *Customer) DisplayName() string {
	if c.Name != nil && *c.Name != "" {
		return *c.Name
	}
	return c.PhoneNumber
}

func (c *Customer) HasName() bool {
	return c.Name != nil && *c.Name != ""
}
