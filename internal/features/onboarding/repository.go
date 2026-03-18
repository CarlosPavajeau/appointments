package onboarding

import (
	"context"
	"time"
	"wappiz/internal/platform/database"

	apperrors "wappiz/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Repository interface {
	FindByTenant(ctx context.Context, tenantID uuid.UUID) (*Progress, error)
	Create(ctx context.Context, tenantID uuid.UUID) (*Progress, error)
	AdvanceStep(ctx context.Context, tenantID uuid.UUID) error
	Complete(ctx context.Context, tenantID uuid.UUID) error
}

type pgRepository struct{ db *sqlx.DB }

func NewRepository(db *sqlx.DB) Repository {
	return &pgRepository{db: db}
}

type progressRow struct {
	ID          uuid.UUID  `db:"id"`
	TenantID    uuid.UUID  `db:"tenant_id"`
	CurrentStep int        `db:"current_step"`
	CompletedAt *time.Time `db:"completed_at"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (r progressRow) toDomain() Progress {
	return Progress{
		ID:          r.ID,
		TenantID:    r.TenantID,
		CurrentStep: Step(r.CurrentStep),
		CompletedAt: r.CompletedAt,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func (r *pgRepository) FindByTenant(ctx context.Context, tenantID uuid.UUID) (*Progress, error) {
	var row progressRow
	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, current_step, completed_at, created_at, updated_at
		FROM onboarding_progress
		WHERE tenant_id = $1
	`, tenantID)

	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return new(row.toDomain()), nil
}

func (r *pgRepository) Create(ctx context.Context, tenantID uuid.UUID) (*Progress, error) {
	var row progressRow
	err := r.db.GetContext(ctx, &row, `
		INSERT INTO onboarding_progress (id, tenant_id, current_step)
		VALUES ($1, $2, $3)
		RETURNING id, tenant_id, current_step, completed_at, created_at, updated_at
	`, uuid.New(), tenantID, StepAccount)

	if err != nil {
		return nil, err
	}

	return new(row.toDomain()), nil
}

func (r *pgRepository) AdvanceStep(ctx context.Context, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE onboarding_progress
		SET
			current_step = current_step + 1,
			updated_at   = NOW()
		WHERE tenant_id = $1
		  AND current_step < $2
	`, tenantID, int(StepWhatsApp))
	return err
}

func (r *pgRepository) Complete(ctx context.Context, tenantID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE onboarding_progress
		SET
			completed_at = NOW(),
			updated_at   = NOW()
		WHERE tenant_id = $1
	`, tenantID)
	return err
}
