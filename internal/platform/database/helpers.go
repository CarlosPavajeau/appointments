package database

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrNoRows) {
		return true
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}

	return strings.Contains(err.Error(), "no rows in result set")
}

func IsConstraintError(err error, constraintName string) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		// 23P01 = exclusion_violation
		// 23505 = unique_violation
		return pgErr.ConstraintName == constraintName
	}
	return false
}

func IsOverlapError(err error) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return pgErr.Code == "23P01" || pgErr.ConstraintName == "no_overlap"
	}
	return strings.Contains(err.Error(), "no_overlap") ||
		strings.Contains(err.Error(), "exclusion constraint")
}
