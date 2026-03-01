package customers

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
	FindOrCreate(ctx context.Context, tenantID uuid.UUID, phone string) (*Customer, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Customer, error)
	FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Customer, error)
	UpdateName(ctx context.Context, id uuid.UUID, name string) error
	Block(ctx context.Context, id, tenantID uuid.UUID) error
	Unblock(ctx context.Context, id, tenantID uuid.UUID) error
}

type pgRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository {
	return &pgRepository{db: db}
}

type customerRow struct {
	ID          uuid.UUID `db:"id"`
	TenantID    uuid.UUID `db:"tenant_id"`
	PhoneNumber string    `db:"phone_number"`
	Name        *string   `db:"name"`
	IsBlocked   bool      `db:"is_blocked"`
	CreatedAt   time.Time `db:"created_at"`
}

func (r customerRow) toDomain() Customer {
	return Customer{
		ID:          r.ID,
		TenantID:    r.TenantID,
		PhoneNumber: r.PhoneNumber,
		Name:        r.Name,
		IsBlocked:   r.IsBlocked,
		CreatedAt:   r.CreatedAt,
	}
}

func (r *pgRepository) FindOrCreate(ctx context.Context, tenantID uuid.UUID, phone string) (*Customer, error) {
	// INSERT ... ON CONFLICT es atómico — evita race conditions si dos mensajes
	// del mismo número llegan al mismo tiempo
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO customers (id, tenant_id, phone_number)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, phone_number) DO NOTHING
	`, uuid.New(), tenantID, phone)
	if err != nil {
		return nil, err
	}

	// Después del upsert, leer el registro final
	var row customerRow
	err = r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, phone_number, name, is_blocked, created_at
		FROM customers
		WHERE tenant_id = $1 AND phone_number = $2
	`, tenantID, phone)
	if err != nil {
		return nil, err
	}

	c := row.toDomain()
	return &c, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Customer, error) {
	var row customerRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, phone_number, name, is_blocked, created_at
		FROM customers
		WHERE id = $1
	`, id)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	c := row.toDomain()
	return &c, nil
}

func (r *pgRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID) ([]Customer, error) {
	var rows []customerRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT id, tenant_id, phone_number, name, is_blocked, created_at
		FROM customers
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`, tenantID)
	if err != nil {
		return nil, err
	}

	result := make([]Customer, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result, nil
}

func (r *pgRepository) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE customers SET name = $1 WHERE id = $2
	`, name, id)
	return err
}

func (r *pgRepository) Block(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE customers SET is_blocked = true
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return err
}

func (r *pgRepository) Unblock(ctx context.Context, id, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE customers SET is_blocked = false
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return err
}
