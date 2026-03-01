package resources

import (
	"context"
	"time"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
)

type UseCases struct {
	repo Repository
}

func NewUseCases(repo Repository) *UseCases {
	return &UseCases{repo: repo}
}

// ── Resource CRUD ─────────────────────────────────────────────────

type CreateResourceInput struct {
	TenantID  uuid.UUID
	Name      string
	Type      ResourceType
	AvatarURL string
}

type UpdateResourceInput struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	Name      string
	Type      ResourceType
	AvatarURL string
	SortOrder int
}

func (uc *UseCases) GetAll(ctx context.Context, tenantID uuid.UUID) ([]Resource, error) {
	return uc.repo.FindByTenant(ctx, tenantID)
}

func (uc *UseCases) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*Resource, error) {
	res, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if res.TenantID != tenantID {
		return nil, apperrors.ErrNotFound
	}
	return res, nil
}

func (uc *UseCases) Create(ctx context.Context, input CreateResourceInput) (*Resource, error) {
	res := &Resource{
		ID:        uuid.New(),
		TenantID:  input.TenantID,
		Name:      input.Name,
		Type:      input.Type,
		AvatarURL: input.AvatarURL,
	}

	if err := res.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.Create(ctx, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (uc *UseCases) Update(ctx context.Context, input UpdateResourceInput) (*Resource, error) {
	existing, err := uc.GetByID(ctx, input.ID, input.TenantID)
	if err != nil {
		return nil, err
	}

	existing.Name = input.Name
	existing.Type = input.Type
	existing.AvatarURL = input.AvatarURL
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

// ── Working Hours ─────────────────────────────────────────────────

type UpsertWorkingHoursInput struct {
	ResourceID uuid.UUID
	TenantID   uuid.UUID
	DayOfWeek  int
	StartTime  string
	EndTime    string
	IsActive   bool
}

func (uc *UseCases) UpsertWorkingHours(ctx context.Context, input UpsertWorkingHoursInput) (*WorkingHours, error) {
	if _, err := uc.GetByID(ctx, input.ResourceID, input.TenantID); err != nil {
		return nil, err
	}

	wh := WorkingHours{
		ID:         uuid.New(),
		ResourceID: input.ResourceID,
		DayOfWeek:  input.DayOfWeek,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		IsActive:   input.IsActive,
	}

	if err := wh.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.UpsertWorkingHours(ctx, wh); err != nil {
		return nil, err
	}
	return &wh, nil
}

func (uc *UseCases) DeleteWorkingHours(ctx context.Context, id, resourceID, tenantID uuid.UUID) error {
	if _, err := uc.GetByID(ctx, resourceID, tenantID); err != nil {
		return err
	}
	return uc.repo.DeleteWorkingHours(ctx, id, resourceID)
}

// ── Schedule Overrides ────────────────────────────────────────────

type CreateOverrideInput struct {
	ResourceID uuid.UUID
	TenantID   uuid.UUID
	Date       time.Time
	IsDayOff   bool
	StartTime  *string
	EndTime    *string
	Reason     string
}

func (uc *UseCases) GetOverrides(ctx context.Context, resourceID, tenantID uuid.UUID, from, to time.Time) ([]ScheduleOverride, error) {
	if _, err := uc.GetByID(ctx, resourceID, tenantID); err != nil {
		return nil, err
	}
	return uc.repo.FindOverrides(ctx, resourceID, from, to)
}

func (uc *UseCases) CreateOverride(ctx context.Context, input CreateOverrideInput) (*ScheduleOverride, error) {
	if _, err := uc.GetByID(ctx, input.ResourceID, input.TenantID); err != nil {
		return nil, err
	}

	so := &ScheduleOverride{
		ID:         uuid.New(),
		ResourceID: input.ResourceID,
		Date:       input.Date,
		IsDayOff:   input.IsDayOff,
		StartTime:  input.StartTime,
		EndTime:    input.EndTime,
		Reason:     input.Reason,
	}

	if err := so.Validate(); err != nil {
		return nil, err
	}

	if err := uc.repo.CreateOverride(ctx, so); err != nil {
		return nil, err
	}
	return so, nil
}

func (uc *UseCases) DeleteOverride(ctx context.Context, id, resourceID, tenantID uuid.UUID) error {
	if _, err := uc.GetByID(ctx, resourceID, tenantID); err != nil {
		return err
	}
	return uc.repo.DeleteOverride(ctx, id, resourceID)
}

// ── Service Assignments ───────────────────────────────────────────

func (uc *UseCases) AssignServices(ctx context.Context, resourceID, tenantID uuid.UUID, serviceIDs []uuid.UUID) error {
	if _, err := uc.GetByID(ctx, resourceID, tenantID); err != nil {
		return err
	}
	return uc.repo.AssignServices(ctx, resourceID, serviceIDs)
}

func (uc *UseCases) GetServiceIDs(ctx context.Context, resourceID, tenantID uuid.UUID) ([]uuid.UUID, error) {
	if _, err := uc.GetByID(ctx, resourceID, tenantID); err != nil {
		return nil, err
	}
	return uc.repo.FindServiceIDs(ctx, resourceID)
}
