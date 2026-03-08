package users

import (
	"appointments/internal/platform/database"
	apperrors "appointments/internal/shared/errors"
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)

	Save(ctx context.Context, u *User) error
}

type pgRepository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) Repository {
	return &pgRepository{
		db: db,
	}
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var row struct {
		ID           uuid.UUID `db:"id"`
		TenantID     uuid.UUID `db:"tenant_id"`
		Email        string    `db:"email"`
		PasswordHash string    `db:"password_hash"`
		Role         string    `db:"role"`
		CreatedAt    time.Time `db:"created_at"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, email, password_hash, role, created_at
		FROM tenant_users WHERE id = $1
	`, id)

	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return &User{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *pgRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var row struct {
		ID           uuid.UUID `db:"id"`
		TenantID     uuid.UUID `db:"tenant_id"`
		Email        string    `db:"email"`
		PasswordHash string    `db:"password_hash"`
		Role         string    `db:"role"`
		CreatedAt    time.Time `db:"created_at"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, email, password_hash, role, created_at
		FROM tenant_users
		WHERE email = $1
	`, email)

	if database.IsNotFound(err) {
		return nil, apperrors.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &User{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *pgRepository) Save(ctx context.Context, u *User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tenant_users (id, tenant_id, email, password_hash, role)
		VALUES ($1,$2,$3,$4,$5)
	`, u.ID, u.TenantID, u.Email, u.PasswordHash, u.Role)

	return err
}
