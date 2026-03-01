package customers

import (
	"context"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
)

type UseCases struct {
	repo Repository
}

func NewUseCases(repo Repository) *UseCases {
	return &UseCases{repo: repo}
}

func (uc *UseCases) ResolveIncoming(ctx context.Context, tenantID uuid.UUID, phone string) (*Customer, error) {
	return uc.repo.FindOrCreate(ctx, tenantID, phone)
}

func (uc *UseCases) GetAll(ctx context.Context, tenantID uuid.UUID) ([]Customer, error) {
	return uc.repo.FindByTenant(ctx, tenantID)
}

func (uc *UseCases) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Customer, error) {
	customer, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if customer.TenantID != tenantID {
		return nil, apperrors.ErrNotFound
	}
	return customer, nil
}

func (uc *UseCases) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	return uc.repo.UpdateName(ctx, id, name)
}

func (uc *UseCases) Block(ctx context.Context, id, tenantID uuid.UUID) error {
	if _, err := uc.GetByID(ctx, id, tenantID); err != nil {
		return err
	}
	return uc.repo.Block(ctx, id, tenantID)
}

func (uc *UseCases) Unblock(ctx context.Context, id, tenantID uuid.UUID) error {
	if _, err := uc.GetByID(ctx, id, tenantID); err != nil {
		return err
	}
	return uc.repo.Unblock(ctx, id, tenantID)
}
