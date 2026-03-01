package services

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

type CreateServiceInput struct {
	TenantID        uuid.UUID
	Name            string
	Description     string
	DurationMinutes int
	BufferMinutes   int
	Price           float64
}

type UpdateServiceInput struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Name            string
	Description     string
	DurationMinutes int
	BufferMinutes   int
	Price           float64
	SortOrder       int
}

func (uc *UseCases) GetAll(ctx context.Context, tenantID uuid.UUID) ([]Service, error) {
	return uc.repo.FindByTenant(ctx, tenantID)
}

func (uc *UseCases) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Service, error) {
	svc, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Avoid cross-tenant access
	if svc.TenantID != tenantID {
		return nil, apperrors.ErrNotFound
	}
	return svc, nil
}

func (uc *UseCases) Create(ctx context.Context, input CreateServiceInput) (*Service, error) {
	svc := &Service{
		ID:              uuid.New(),
		TenantID:        input.TenantID,
		Name:            input.Name,
		Description:     input.Description,
		DurationMinutes: input.DurationMinutes,
		BufferMinutes:   input.BufferMinutes,
		Price:           input.Price,
	}

	if err := svc.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.Create(ctx, svc); err != nil {
		return nil, err
	}
	return svc, nil
}

func (uc *UseCases) Update(ctx context.Context, input UpdateServiceInput) (*Service, error) {
	existing, err := uc.GetByID(ctx, input.ID, input.TenantID)
	if err != nil {
		return nil, err
	}

	existing.Name = input.Name
	existing.Description = input.Description
	existing.DurationMinutes = input.DurationMinutes
	existing.BufferMinutes = input.BufferMinutes
	existing.Price = input.Price
	existing.SortOrder = input.SortOrder

	if err := existing.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (uc *UseCases) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	// Check existence and tenant ownership before deleting
	if _, err := uc.GetByID(ctx, id, tenantID); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id, tenantID)
}

func (uc *UseCases) UpdateSortOrder(ctx context.Context, tenantID uuid.UUID, order []SortItem) error {
	if len(order) == 0 {
		return nil
	}
	return uc.repo.UpdateSortOrder(ctx, tenantID, order)
}
