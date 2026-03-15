package resources

import (
	"context"
	"wappiz/internal/platform/database"

	"time"

	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Resource, error)
	FindByTenantAndService(ctx context.Context, tenantID, serviceID uuid.UUID) ([]Resource, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Resource, error)
	Create(ctx context.Context, r *Resource) error
	Update(ctx context.Context, r *Resource) error
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	UpdateSortOrder(ctx context.Context, tenantID uuid.UUID, order []SortItem) error

	FindWorkingHours(ctx context.Context, resourceID uuid.UUID) ([]WorkingHours, error)
	UpsertWorkingHours(ctx context.Context, wh WorkingHours) error
	DeleteWorkingHours(ctx context.Context, id, resourceID uuid.UUID) error

	FindOverrides(ctx context.Context, resourceID uuid.UUID, from, to time.Time) ([]ScheduleOverride, error)
	FindOverrideByDate(ctx context.Context, resourceID uuid.UUID, date time.Time) (*ScheduleOverride, error)
	CreateOverride(ctx context.Context, so *ScheduleOverride) error
	DeleteOverride(ctx context.Context, id, resourceID uuid.UUID) error

	AssignServices(ctx context.Context, resourceID uuid.UUID, serviceIDs []uuid.UUID) error
	FindServiceIDs(ctx context.Context, resourceID uuid.UUID) ([]uuid.UUID, error)
}

type SortItem struct {
	ID        uuid.UUID
	SortOrder int
}

type pgRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository {
	return &pgRepository{db: db}
}

type resourceRow struct {
	ID        uuid.UUID `db:"id"`
	TenantID  uuid.UUID `db:"tenant_id"`
	Name      string    `db:"name"`
	Type      string    `db:"type"`
	AvatarURL string    `db:"avatar_url"`
	IsActive  bool      `db:"is_active"`
	SortOrder int       `db:"sort_order"`
	CreatedAt time.Time `db:"created_at"`
}

func (r resourceRow) toDomain() Resource {
	return Resource{
		ID:        r.ID,
		TenantID:  r.TenantID,
		Name:      r.Name,
		Type:      ResourceType(r.Type),
		AvatarURL: r.AvatarURL,
		IsActive:  r.IsActive,
		SortOrder: r.SortOrder,
		CreatedAt: r.CreatedAt,
	}
}

type workingHoursRow struct {
	ID         uuid.UUID `db:"id"`
	ResourceID uuid.UUID `db:"resource_id"`
	DayOfWeek  int       `db:"day_of_week"`
	StartTime  string    `db:"start_time"`
	EndTime    string    `db:"end_time"`
	IsActive   bool      `db:"is_active"`
}

func (r workingHoursRow) toDomain() WorkingHours {
	return WorkingHours{
		ID:         r.ID,
		ResourceID: r.ResourceID,
		DayOfWeek:  r.DayOfWeek,
		StartTime:  r.StartTime,
		EndTime:    r.EndTime,
		IsActive:   r.IsActive,
	}
}

type scheduleOverrideRow struct {
	ID         uuid.UUID `db:"id"`
	ResourceID uuid.UUID `db:"resource_id"`
	Date       time.Time `db:"date"`
	IsDayOff   bool      `db:"is_day_off"`
	StartTime  *string   `db:"start_time"`
	EndTime    *string   `db:"end_time"`
	Reason     string    `db:"reason"`
	CreatedAt  time.Time `db:"created_at"`
}

func (r scheduleOverrideRow) toDomain() ScheduleOverride {
	return ScheduleOverride{
		ID:         r.ID,
		ResourceID: r.ResourceID,
		Date:       r.Date,
		IsDayOff:   r.IsDayOff,
		StartTime:  r.StartTime,
		EndTime:    r.EndTime,
		Reason:     r.Reason,
		CreatedAt:  r.CreatedAt,
	}
}

// ── Resources ─────────────────────────────────────────────────────

func (r *pgRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Resource, error) {
	var rows []resourceRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, tenant_id, name, type, COALESCE(avatar_url, '') as avatar_url,
		       is_active, sort_order, created_at
		FROM resources
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY sort_order ASC, created_at ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(rows))
	for i, row := range rows {
		res := row.toDomain()
		wh, _ := r.FindWorkingHours(ctx, res.ID)
		res.WorkingHours = wh
		result[i] = res
	}
	return result, nil
}

func (r *pgRepository) FindByTenantAndService(ctx context.Context, tenantID, serviceID uuid.UUID) ([]Resource, error) {
	var rows []resourceRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT r.id, r.tenant_id, r.name, r.type,
		       COALESCE(r.avatar_url, '') as avatar_url,
		       r.is_active, r.sort_order, r.created_at
		FROM resources r
		JOIN resource_services rs ON rs.resource_id = r.id
		WHERE r.tenant_id = $1
		  AND rs.service_id = $2
		  AND r.is_active = true
		ORDER BY r.sort_order ASC
	`, tenantID, serviceID)
	if err != nil {
		return nil, err
	}

	result := make([]Resource, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Resource, error) {
	var row resourceRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, name, type, COALESCE(avatar_url, '') as avatar_url,
		       is_active, sort_order, created_at
		FROM resources
		WHERE id = $1 AND is_active = true
	`, id)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	res := row.toDomain()
	wh, _ := r.FindWorkingHours(ctx, res.ID)
	res.WorkingHours = wh
	return &res, nil
}

func (r *pgRepository) Create(ctx context.Context, res *Resource) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO resources
			(id, tenant_id, name, type, avatar_url, is_active, sort_order)
		VALUES ($1,$2,$3,$4,$5,true,$6)
	`, res.ID, res.TenantID, res.Name, string(res.Type),
		nullableString(res.AvatarURL), res.SortOrder)
	return err
}

func (r *pgRepository) Update(ctx context.Context, res *Resource) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE resources
		SET name = $1, type = $2, avatar_url = $3, sort_order = $4
		WHERE id = $5 AND tenant_id = $6
	`, res.Name, string(res.Type), nullableString(res.AvatarURL),
		res.SortOrder, res.ID, res.TenantID)
	return err
}

func (r *pgRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE resources SET is_active = false
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return err
}

func (r *pgRepository) UpdateSortOrder(ctx context.Context, tenantID uuid.UUID, order []SortItem) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, item := range order {
		_, err := tx.ExecContext(ctx, `
			UPDATE resources SET sort_order = $1
			WHERE id = $2 AND tenant_id = $3
		`, item.SortOrder, item.ID, tenantID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Working Hours ─────────────────────────────────────────────────

func (r *pgRepository) FindWorkingHours(ctx context.Context, resourceID uuid.UUID) ([]WorkingHours, error) {
	var rows []workingHoursRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, resource_id, day_of_week,
		       start_time::text, end_time::text, is_active
		FROM working_hours
		WHERE resource_id = $1
		ORDER BY day_of_week ASC
	`, resourceID)
	if err != nil {
		return nil, err
	}

	result := make([]WorkingHours, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) UpsertWorkingHours(ctx context.Context, wh WorkingHours) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO working_hours
			(id, resource_id, day_of_week, start_time, end_time, is_active)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (resource_id, day_of_week)
		DO UPDATE SET
			start_time = EXCLUDED.start_time,
			end_time   = EXCLUDED.end_time,
			is_active  = EXCLUDED.is_active
	`, wh.ID, wh.ResourceID, wh.DayOfWeek, wh.StartTime, wh.EndTime, wh.IsActive)
	return err
}

func (r *pgRepository) DeleteWorkingHours(ctx context.Context, id, resourceID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM working_hours WHERE id = $1 AND resource_id = $2
	`, id, resourceID)
	return err
}

// ── Schedule Overrides ────────────────────────────────────────────

func (r *pgRepository) FindOverrides(ctx context.Context, resourceID uuid.UUID, from, to time.Time) ([]ScheduleOverride, error) {
	var rows []scheduleOverrideRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, resource_id, date, is_day_off,
		       start_time::text, end_time::text,
		       COALESCE(reason, '') as reason, created_at
		FROM schedule_overrides
		WHERE resource_id = $1
		  AND date BETWEEN $2 AND $3
		ORDER BY date ASC
	`, resourceID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}

	result := make([]ScheduleOverride, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) FindOverrideByDate(ctx context.Context, resourceID uuid.UUID, date time.Time) (*ScheduleOverride, error) {
	var row scheduleOverrideRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, resource_id, date, is_day_off,
		       start_time::text, end_time::text,
		       COALESCE(reason, '') as reason, created_at
		FROM schedule_overrides
		WHERE resource_id = $1 AND date = $2
		LIMIT 1
	`, resourceID, date.Format("2006-01-02"))

	if database.IsNotFound(err) {
		return nil, nil // no override, valid day
	}
	if err != nil {
		return nil, err
	}

	so := row.toDomain()
	return &so, nil
}

func (r *pgRepository) CreateOverride(ctx context.Context, so *ScheduleOverride) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO schedule_overrides
			(id, resource_id, date, is_day_off, start_time, end_time, reason)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (resource_id, date)
		DO UPDATE SET
			is_day_off = EXCLUDED.is_day_off,
			start_time = EXCLUDED.start_time,
			end_time   = EXCLUDED.end_time,
			reason     = EXCLUDED.reason
	`, so.ID, so.ResourceID, so.Date.Format("2006-01-02"),
		so.IsDayOff, so.StartTime, so.EndTime, so.Reason)
	return err
}

func (r *pgRepository) DeleteOverride(ctx context.Context, id, resourceID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM schedule_overrides WHERE id = $1 AND resource_id = $2
	`, id, resourceID)
	return err
}

// ── Resource Services ─────────────────────────────────────────────

func (r *pgRepository) AssignServices(ctx context.Context, resourceID uuid.UUID, serviceIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM resource_services WHERE resource_id = $1
	`, resourceID); err != nil {
		return err
	}

	for _, svcID := range serviceIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO resource_services (resource_id, service_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING
		`, resourceID, svcID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *pgRepository) FindServiceIDs(ctx context.Context, resourceID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.SelectContext(ctx, &ids, `
		SELECT service_id FROM resource_services WHERE resource_id = $1
	`, resourceID)
	return ids, err
}

// ── Helpers ───────────────────────────────────────────────────────

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
