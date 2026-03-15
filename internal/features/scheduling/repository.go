package scheduling

import (
	"context"
	"encoding/json"
	"time"
	"wappiz/internal/platform/database"

	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type SessionRepository interface {
	FindActive(ctx context.Context, tenantID, customerID uuid.UUID) (*Session, error)
	Create(ctx context.Context, session *Session) error
	Update(ctx context.Context, session *Session) error
	Delete(ctx context.Context, sessionID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type AvailabilityRepository interface {
	GetWorkingHours(ctx context.Context, resourceID uuid.UUID) ([]WorkingHours, error)
	GetOverrides(ctx context.Context, resourceID uuid.UUID, date time.Time) (*ScheduleOverride, error)
	GetOccupiedSlots(ctx context.Context, resourceID uuid.UUID, date time.Time) ([]TimeSlot, error)
}

type pgSessionRepository struct{ db *sqlx.DB }

func NewSessionRepository(db *sqlx.DB) SessionRepository {
	return &pgSessionRepository{db: db}
}

type sessionRow struct {
	ID               uuid.UUID `db:"id"`
	TenantID         uuid.UUID `db:"tenant_id"`
	WhatsappConfigID uuid.UUID `db:"whatsapp_config_id"`
	CustomerID       uuid.UUID `db:"customer_id"`
	Step             string    `db:"step"`
	Data             []byte    `db:"data"`
	ExpiresAt        time.Time `db:"expires_at"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

func (r *pgSessionRepository) FindActive(ctx context.Context, tenantID, customerID uuid.UUID) (*Session, error) {
	var row sessionRow
	err := r.db.GetContext(ctx, &row, `
        SELECT id, tenant_id, whatsapp_config_id, customer_id, step, data, expires_at, created_at, updated_at
        FROM conversation_sessions
        WHERE tenant_id = $1 AND customer_id = $2 AND expires_at > NOW()
        LIMIT 1
    `, tenantID, customerID)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	return rowToSession(row)
}

func (r *pgSessionRepository) Create(ctx context.Context, s *Session) error {
	data, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
        INSERT INTO conversation_sessions
            (id, tenant_id, whatsapp_config_id, customer_id, step, data, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (tenant_id, customer_id) DO UPDATE
            SET step = EXCLUDED.step,
                data = EXCLUDED.data,
                expires_at = EXCLUDED.expires_at,
                updated_at = NOW()
    `, s.ID, s.TenantID, s.WhatsappConfigID, s.CustomerID, string(s.Step), data, s.ExpiresAt)

	return err
}

func (r *pgSessionRepository) Update(ctx context.Context, s *Session) error {
	data, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
        UPDATE conversation_sessions
        SET step = $1, data = $2, expires_at = $3, updated_at = NOW()
        WHERE id = $4
    `, string(s.Step), data, s.ExpiresAt, s.ID)

	return err
}

func (r *pgSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM conversation_sessions WHERE id = $1`, id)
	return err
}

func (r *pgSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM conversation_sessions WHERE expires_at < NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func rowToSession(row sessionRow) (*Session, error) {
	var data SessionData
	if err := json.Unmarshal(row.Data, &data); err != nil {
		return nil, err
	}
	return &Session{
		ID:               row.ID,
		TenantID:         row.TenantID,
		WhatsappConfigID: row.WhatsappConfigID,
		CustomerID:       row.CustomerID,
		Step:             SessionStep(row.Step),
		Data:             data,
		ExpiresAt:        row.ExpiresAt,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, nil
}

type pgAvailabilityRepository struct{ db *sqlx.DB }

func NewAvailabilityRepository(db *sqlx.DB) AvailabilityRepository {
	return &pgAvailabilityRepository{db: db}
}

func (r *pgAvailabilityRepository) GetWorkingHours(ctx context.Context, resourceID uuid.UUID) ([]WorkingHours, error) {
	var rows []struct {
		DayOfWeek int    `db:"day_of_week"`
		StartTime string `db:"start_time"`
		EndTime   string `db:"end_time"`
	}
	err := r.db.SelectContext(ctx, &rows, `
        SELECT day_of_week, start_time::text, end_time::text
        FROM working_hours
        WHERE resource_id = $1 AND is_active = true
        ORDER BY day_of_week
    `, resourceID)
	if err != nil {
		return nil, err
	}
	result := make([]WorkingHours, len(rows))
	for i, r := range rows {
		result[i] = WorkingHours{DayOfWeek: r.DayOfWeek, StartTime: r.StartTime, EndTime: r.EndTime}
	}
	return result, nil
}

func (r *pgAvailabilityRepository) GetOverrides(ctx context.Context, resourceID uuid.UUID, date time.Time) (*ScheduleOverride, error) {
	var row struct {
		IsDayOff  bool    `db:"is_day_off"`
		StartTime *string `db:"start_time"`
		EndTime   *string `db:"end_time"`
	}
	err := r.db.GetContext(ctx, &row, `
        SELECT is_day_off, start_time::text, end_time::text
        FROM schedule_overrides
        WHERE resource_id = $1 AND date = $2
        LIMIT 1
    `, resourceID, date.Format("2006-01-02"))

	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ScheduleOverride{
		Date: date, IsDayOff: row.IsDayOff,
		StartTime: row.StartTime, EndTime: row.EndTime,
	}, nil
}

func (r *pgAvailabilityRepository) GetOccupiedSlots(ctx context.Context, resourceID uuid.UUID, date time.Time) ([]TimeSlot, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)

	var rows []struct {
		StartsAt time.Time `db:"starts_at"`
		EndsAt   time.Time `db:"ends_at"`
	}
	err := r.db.SelectContext(ctx, &rows, `
        SELECT starts_at, ends_at
        FROM appointments
        WHERE resource_id = $1
          AND starts_at >= $2 AND ends_at <= $3
          AND status NOT IN ('cancelled')
        ORDER BY starts_at
    `, resourceID, start, end)

	if err != nil {
		return nil, err
	}

	result := make([]TimeSlot, len(rows))
	for i, row := range rows {
		result[i] = TimeSlot{StartsAt: row.StartsAt, EndsAt: row.EndsAt, ResourceID: resourceID}
	}
	return result, nil
}
