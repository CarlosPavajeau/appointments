package db

import (
	"context"
	"database/sql"
)

type Replica struct {
	db *sql.DB
}

var _ DBTX = (*Replica)(nil)

func (r *Replica) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := r.db.ExecContext(ctx, query, args...)

	return result, err
}

func (r *Replica) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := r.db.PrepareContext(ctx, query)

	return stmt, err
}

func (r *Replica) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)

	return rows, err
}

func (r *Replica) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	row := r.db.QueryRowContext(ctx, query, args...)

	return row
}

func (r *Replica) Begin(ctx context.Context) (DBTx, error) {
	tx, err := r.db.BeginTx(ctx, nil)

	return tx, err
}
