package tenants

import (
	database "appointments/internal/platform/database"
	"context"
	"time"

	apperrors "appointments/internal/shared/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RefreshToken struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

func (rt *RefreshToken) IsRevoked() bool {
	return rt.RevokedAt != nil
}

type RefreshTokenRepository interface {
	Create(ctx context.Context, rt *RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*RefreshToken, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type pgRefreshTokenRepository struct{ db *sqlx.DB }

func NewRefreshTokenRepository(db *sqlx.DB) RefreshTokenRepository {
	return &pgRefreshTokenRepository{db: db}
}

func (r *pgRefreshTokenRepository) Create(ctx context.Context, rt *RefreshToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens
			(id, tenant_id, user_id, token_hash, family_id, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)
	`, rt.ID, rt.TenantID, rt.UserID, rt.TokenHash, rt.FamilyID, rt.ExpiresAt)
	return err
}

func (r *pgRefreshTokenRepository) FindByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	var row struct {
		ID        uuid.UUID  `db:"id"`
		TenantID  uuid.UUID  `db:"tenant_id"`
		UserID    uuid.UUID  `db:"user_id"`
		TokenHash string     `db:"token_hash"`
		FamilyID  uuid.UUID  `db:"family_id"`
		ExpiresAt time.Time  `db:"expires_at"`
		RevokedAt *time.Time `db:"revoked_at"`
		CreatedAt time.Time  `db:"created_at"`
	}

	err := r.db.GetContext(ctx, &row, `
		SELECT id, tenant_id, user_id, token_hash, family_id,
		       expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`, hash)

	if err != nil {
		if database.IsNotFound(err) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	return &RefreshToken{
		ID:        row.ID,
		TenantID:  row.TenantID,
		UserID:    row.UserID,
		TokenHash: row.TokenHash,
		FamilyID:  row.FamilyID,
		ExpiresAt: row.ExpiresAt,
		RevokedAt: row.RevokedAt,
		CreatedAt: row.CreatedAt,
	}, nil
}

func (r *pgRefreshTokenRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1
	`, id)
	return err
}

func (r *pgRefreshTokenRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE family_id = $1 AND revoked_at IS NULL
	`, familyID)
	return err
}

func (r *pgRefreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM refresh_tokens
		WHERE expires_at < NOW() OR revoked_at IS NOT NULL
	`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
