package services

import (
	"context"
	"errors"
	"time"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Service, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Service, error)
	FindByTenantAndResource(ctx context.Context, tenantID, resourceID uuid.UUID) ([]Service, error)
	Create(ctx context.Context, s *Service) error
	Update(ctx context.Context, s *Service) error
	Delete(ctx context.Context, id, tenantID uuid.UUID) error
	UpdateSortOrder(ctx context.Context, tenantID uuid.UUID, order []SortItem) error
}

type SortItem struct {
	ID        uuid.UUID
	SortOrder int
}

type pgRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository {
	return &pgRepository{db: db}
}

type serviceRow struct {
	ID              uuid.UUID `db:"id"`
	TenantID        uuid.UUID `db:"tenant_id"`
	Name            string    `db:"name"`
	Description     string    `db:"description"`
	DurationMinutes int       `db:"duration_minutes"`
	BufferMinutes   int       `db:"buffer_minutes"`
	Price           float64   `db:"price"`
	IsActive        bool      `db:"is_active"`
	SortOrder       int       `db:"sort_order"`
	CreatedAt       time.Time `db:"created_at"`
}

func (r serviceRow) toDomain() Service {
	return Service{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Name:            r.Name,
		Description:     r.Description,
		DurationMinutes: r.DurationMinutes,
		BufferMinutes:   r.BufferMinutes,
		Price:           r.Price,
		IsActive:        r.IsActive,
		SortOrder:       r.SortOrder,
		CreatedAt:       r.CreatedAt,
	}
}

func (r *pgRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Service, error) {
	var rows []serviceRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, tenant_id, name, description, duration_minutes,
		       buffer_minutes, price, is_active, sort_order, created_at
		FROM services
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY sort_order ASC, created_at ASC
	`, tenantID)
	if err != nil {
		return nil, err
	}

	result := make([]Service, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Service, error) {
	var row serviceRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, name, description, duration_minutes,
		       buffer_minutes, price, is_active, sort_order, created_at
		FROM services
		WHERE id = $1 AND is_active = true
	`, id)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return new(row.toDomain()), nil
}

func (r *pgRepository) FindByTenantAndResource(ctx context.Context, tenantID, resourceID uuid.UUID) ([]Service, error) {
	var rows []serviceRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT s.id, s.tenant_id, s.name, s.description, s.duration_minutes,
		       s.buffer_minutes, s.price, s.is_active, s.sort_order, s.created_at
		FROM services s
		JOIN resource_services rs ON rs.service_id = s.id
		WHERE s.tenant_id = $1
		  AND rs.resource_id = $2
		  AND s.is_active = true
		ORDER BY s.sort_order ASC
	`, tenantID, resourceID)
	if err != nil {
		return nil, err
	}

	result := make([]Service, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) Create(ctx context.Context, s *Service) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO services
			(id, tenant_id, name, description, duration_minutes,
			 buffer_minutes, price, is_active, sort_order)
		VALUES ($1,$2,$3,$4,$5,$6,$7,true,$8)
	`, s.ID, s.TenantID, s.Name, s.Description,
		s.DurationMinutes, s.BufferMinutes, s.Price, s.SortOrder)
	return err
}

func (r *pgRepository) Update(ctx context.Context, s *Service) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE services
		SET name = $1, description = $2, duration_minutes = $3,
		    buffer_minutes = $4, price = $5, sort_order = $6
		WHERE id = $7 AND tenant_id = $8
	`, s.Name, s.Description, s.DurationMinutes,
		s.BufferMinutes, s.Price, s.SortOrder, s.ID, s.TenantID)
	return err
}

func (r *pgRepository) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	// Soft delete
	_, err := r.db.ExecContext(ctx, `
		UPDATE services
		SET is_active = false
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
			UPDATE services
			SET sort_order = $1
			WHERE id = $2 AND tenant_id = $3
		`, item.SortOrder, item.ID, tenantID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
