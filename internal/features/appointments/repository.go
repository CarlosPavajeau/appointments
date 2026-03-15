package appointments

import (
	"appointments/internal/platform/database"
	apperrors "appointments/internal/shared/errors"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	Save(ctx context.Context, a *Appointment) error

	FindByID(ctx context.Context, id, tenantID uuid.UUID) (*Appointment, error)
	FindByCustomerID(ctx context.Context, tenantID, customerID uuid.UUID) ([]Appointment, error)
	FindUpcomingForReminders(ctx context.Context) ([]Appointment, error)
	FindByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]Appointment, error)
	FindByDateWithDetails(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]AppointmentWithDetails, error)

	UpdateStatus(ctx context.Context, id uuid.UUID, status, cancelledBy, reason string) error
	MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error
}

type pgAppointmentRepository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &pgAppointmentRepository{db: db}
}

func (r *pgAppointmentRepository) Save(ctx context.Context, a *Appointment) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO appointments
            (id, tenant_id, resource_id, service_id, customer_id,
             starts_at, ends_at, status, price_at_booking)
        VALUES ($1,$2,$3,$4,$5,$6,$7,'confirmed',$8)
    `, a.ID, a.TenantID, a.ResourceID, a.ServiceID, a.CustomerID,
		a.StartsAt, a.EndsAt, a.PriceAtBooking)
	return err
}

func (r *pgAppointmentRepository) FindByID(ctx context.Context, id, tenantID uuid.UUID) (*Appointment, error) {
	var row struct {
		ID             uuid.UUID `db:"id"`
		TenantID       uuid.UUID `db:"tenant_id"`
		ResourceID     uuid.UUID `db:"resource_id"`
		ServiceID      uuid.UUID `db:"service_id"`
		CustomerID     uuid.UUID `db:"customer_id"`
		StartsAt       time.Time `db:"starts_at"`
		EndsAt         time.Time `db:"ends_at"`
		Status         string    `db:"status"`
		PriceAtBooking float64   `db:"price_at_booking"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, resource_id, service_id, customer_id,
		       starts_at, ends_at, status, price_at_booking
		FROM appointments
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)

	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return &Appointment{
		ID:             row.ID,
		TenantID:       row.TenantID,
		ResourceID:     row.ResourceID,
		ServiceID:      row.ServiceID,
		CustomerID:     row.CustomerID,
		StartsAt:       row.StartsAt,
		EndsAt:         row.EndsAt,
		Status:         row.Status,
		PriceAtBooking: row.PriceAtBooking,
	}, nil
}

func (r *pgAppointmentRepository) FindByCustomerID(ctx context.Context, tenantID, customerID uuid.UUID) ([]Appointment, error) {
	var rows []struct {
		ID             uuid.UUID `db:"id"`
		ResourceID     uuid.UUID `db:"resource_id"`
		ServiceID      uuid.UUID `db:"service_id"`
		CustomerID     uuid.UUID `db:"customer_id"`
		StartsAt       time.Time `db:"starts_at"`
		EndsAt         time.Time `db:"ends_at"`
		Status         string    `db:"status"`
		PriceAtBooking float64   `db:"price_at_booking"`
	}

	err := r.db.SelectContext(ctx, &rows, `
        SELECT id, resource_id, service_id, customer_id,
               starts_at, ends_at, status, price_at_booking
        FROM appointments
        WHERE tenant_id = $1 AND customer_id = $2
          AND status = 'confirmed'
          AND starts_at > NOW()
        ORDER BY starts_at ASC
        LIMIT 5
    `, tenantID, customerID)

	if err != nil {
		return nil, err
	}

	result := make([]Appointment, len(rows))
	for i, row := range rows {
		result[i] = Appointment{
			ID: row.ID, ResourceID: row.ResourceID,
			ServiceID: row.ServiceID, CustomerID: row.CustomerID,
			StartsAt: row.StartsAt, EndsAt: row.EndsAt,
			Status: row.Status, PriceAtBooking: row.PriceAtBooking,
		}
	}
	return result, nil
}

func (r *pgAppointmentRepository) FindUpcomingForReminders(ctx context.Context) ([]Appointment, error) {
	var rows []struct {
		ID         uuid.UUID `db:"id"`
		TenantID   uuid.UUID `db:"tenant_id"`
		CustomerID uuid.UUID `db:"customer_id"`
		ResourceID uuid.UUID `db:"resource_id"`
		ServiceID  uuid.UUID `db:"service_id"`
		StartsAt   time.Time `db:"starts_at"`
		EndsAt     time.Time `db:"ends_at"`
	}

	err := r.db.SelectContext(ctx, &rows, `
        SELECT id, tenant_id, customer_id, resource_id, service_id, starts_at, ends_at
        FROM appointments
        WHERE status = 'confirmed'
          AND (
            (reminder_24h_sent_at IS NULL AND starts_at BETWEEN NOW() + INTERVAL '23 hours' AND NOW() + INTERVAL '25 hours')
            OR
            (reminder_1h_sent_at IS NULL AND starts_at BETWEEN NOW() + INTERVAL '50 minutes' AND NOW() + INTERVAL '70 minutes')
          )
    `)

	if err != nil {
		return nil, err
	}

	result := make([]Appointment, len(rows))
	for i, row := range rows {
		result[i] = Appointment{
			ID: row.ID, TenantID: row.TenantID,
			CustomerID: row.CustomerID, ResourceID: row.ResourceID,
			ServiceID: row.ServiceID, StartsAt: row.StartsAt, EndsAt: row.EndsAt,
		}
	}
	return result, nil
}

func (r *pgAppointmentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status, cancelledBy, reason string) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE appointments
        SET status = $1, cancelled_by = $2, cancel_reason = $3, updated_at = NOW()
        WHERE id = $4
    `, status, cancelledBy, reason, id)
	return err
}

func (r *pgAppointmentRepository) MarkReminderSent(ctx context.Context, id uuid.UUID, reminderType string) error {
	col := "reminder_24h_sent_at"
	if reminderType == "1h" {
		col = "reminder_1h_sent_at"
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE appointments SET `+col+` = NOW(), updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *pgAppointmentRepository) FindByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]Appointment, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var rows []struct {
		ID             uuid.UUID `db:"id"`
		TenantID       uuid.UUID `db:"tenant_id"`
		ResourceID     uuid.UUID `db:"resource_id"`
		ServiceID      uuid.UUID `db:"service_id"`
		CustomerID     uuid.UUID `db:"customer_id"`
		StartsAt       time.Time `db:"starts_at"`
		EndsAt         time.Time `db:"ends_at"`
		Status         string    `db:"status"`
		PriceAtBooking float64   `db:"price_at_booking"`
	}

	err := r.db.SelectContext(ctx, &rows, `
        SELECT id, tenant_id, resource_id, service_id, customer_id,
               starts_at, ends_at, status, price_at_booking
        FROM appointments
        WHERE tenant_id = $1
          AND starts_at >= $2 AND starts_at < $3
          AND status != 'cancelled'
        ORDER BY starts_at ASC
    `, tenantID, dayStart, dayEnd)

	if err != nil {
		return nil, err
	}

	result := make([]Appointment, len(rows))
	for i, row := range rows {
		result[i] = Appointment{
			ID:             row.ID,
			TenantID:       row.TenantID,
			ResourceID:     row.ResourceID,
			ServiceID:      row.ServiceID,
			CustomerID:     row.CustomerID,
			StartsAt:       row.StartsAt,
			EndsAt:         row.EndsAt,
			Status:         row.Status,
			PriceAtBooking: row.PriceAtBooking,
		}
	}
	return result, nil
}

func (r *pgAppointmentRepository) FindByDateWithDetails(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]AppointmentWithDetails, error) {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	var rows []struct {
		ID             uuid.UUID `db:"id"`
		StartsAt       time.Time `db:"starts_at"`
		EndsAt         time.Time `db:"ends_at"`
		Status         string    `db:"status"`
		PriceAtBooking float64   `db:"price_at_booking"`
		ResourceName   string    `db:"resource_name"`
		ServiceName    string    `db:"service_name"`
		CustomerName   string    `db:"customer_name"`
	}

	err := r.db.SelectContext(ctx, &rows, `
        SELECT a.id, a.starts_at, a.ends_at, a.status, a.price_at_booking,
               r.name AS resource_name,
               s.name AS service_name,
               COALESCE(c.name, c.phone_number) AS customer_name
        FROM appointments a
        JOIN resources  r ON r.id = a.resource_id
        JOIN services   s ON s.id = a.service_id
        JOIN customers  c ON c.id = a.customer_id
        WHERE a.tenant_id = $1
          AND a.starts_at >= $2 AND a.starts_at < $3
          AND a.status != 'cancelled'
        ORDER BY a.starts_at ASC
    `, tenantID, dayStart, dayEnd)

	if err != nil {
		return nil, err
	}

	result := make([]AppointmentWithDetails, len(rows))
	for i, row := range rows {
		result[i] = AppointmentWithDetails{
			ID:             row.ID,
			StartsAt:       row.StartsAt,
			EndsAt:         row.EndsAt,
			Status:         row.Status,
			PriceAtBooking: row.PriceAtBooking,
			ResourceName:   row.ResourceName,
			ServiceName:    row.ServiceName,
			CustomerName:   row.CustomerName,
		}
	}
	return result, nil
}
